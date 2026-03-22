package fetcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/knoniewicz/pularis/config"
	"github.com/knoniewicz/pularis/internal/httpclient"
	"github.com/knoniewicz/pularis/internal/redispub"

	"github.com/antonmedv/expr/vm"
	"github.com/google/uuid"
)

type Config struct {
	Name                string                 `toml:"name"`
	EventName           string                 `toml:"event_name"`
	Type                string                 `toml:"type"`
	Endpoint            string                 `toml:"endpoint"`
	PollIntervalSeconds int                    `toml:"poll_interval_seconds"`
	Transform           map[string]interface{} `toml:"transform"`
	Headers             map[string]string      `toml:"headers"`
	Events              []EventConfig          `toml:"events"`
	AuthFile            string                 `toml:"auth_file"`
}

type Fetcher struct {
	Name         string
	Cfg          Config
	client       *httpclient.Client
	redis        *redispub.Client
	ctx          context.Context
	headers      map[string]string
	lastPayload  map[string]map[string]interface{}
	eventPool    chan *Event
	workers      int
	wg           sync.WaitGroup
	exprPrograms map[string]*vm.Program
	mutex        sync.RWMutex
}

func (f *Fetcher) RefreshAuthHeaders(auth map[string]interface{}) {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	for key, value := range f.Cfg.Headers {
		processedValue := value
		for tokenKey, tokenValue := range auth {
			placeholder := PlaceholderStart + tokenKey + PlaceholderEnd
			tokenStr, ok := tokenValue.(string)
			if !ok {
				slog.Warn("token value is not a string, skipping", "key", tokenKey, "type", fmt.Sprintf("%T", tokenValue))
				continue
			}
			processedValue = strings.ReplaceAll(processedValue, placeholder, tokenStr)
		}
		f.headers[key] = processedValue
	}
}

func NewFetcher(ctx context.Context, auth map[string]interface{}, client *httpclient.Client, redis *redispub.Client, cfg Config, pularisCfg config.Config) *Fetcher {
	f := &Fetcher{
		Name:         cfg.Name,
		Cfg:          cfg,
		client:       client,
		redis:        redis,
		ctx:          ctx,
		eventPool:    make(chan *Event, pularisCfg.MaxWorkers),
		workers:      pularisCfg.MaxWorkers,
		exprPrograms: make(map[string]*vm.Program),
	}
	f.lastPayload = make(map[string]map[string]interface{})
	f.headers = make(map[string]string)
	f.RefreshAuthHeaders(auth)
	f.initWorkers(ctx)

	return f
}

func (f *Fetcher) Fetch() error {
	allDisabled := true
	for _, event := range f.Cfg.Events {
		if !event.Disabled {
			allDisabled = false
			break
		}
	}
	if allDisabled {
		return nil
	}

	switch f.Cfg.Type {
	case TypeAPI:
		return f.FetchApi()
	default:
		return fmt.Errorf("unsupported type: %s", f.Cfg.Type)
	}
}

func (f *Fetcher) FetchApi() error {
	ctx, cancel := context.WithTimeout(f.ctx, 5*time.Second)
	defer cancel()

	f.mutex.RLock()
	headers := make(map[string]string, len(f.headers))
	for k, v := range f.headers {
		headers[k] = v
	}
	f.mutex.RUnlock()

	body, err := f.client.Get(ctx, f.Cfg.Endpoint, headers)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %v", f.Cfg.Endpoint, err)
	}

	for _, event := range f.Cfg.Events {
		f.eventPool <- &Event{
			EventCfg: event,
			Body:     body,
		}
	}
	return nil
}

func (f *Fetcher) processEvent(event *Event) error {
	if event.EventCfg.Disabled {
		return nil
	}

	values, err := f.transform(event.Body, f.Cfg.Name, event.EventCfg.Transform)
	if err != nil {
		return fmt.Errorf("failed to transform %s: %v", event.EventCfg.Name, err)
	}

	if event.EventCfg.IsSession {
		return f.handleSession(values, event.EventCfg)
	} else {
		return f.handleMetric(values, event.EventCfg)
	}
}

func (f *Fetcher) publish(values map[string]interface{}, eventConfig EventConfig, action string) error {
	payload := map[string]interface{}{
		FieldRequestID: uuid.New().String(),
		FieldPayload:   values,
	}
	json, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal payload: %v", err)
	}
	f.mutex.Lock()
	f.lastPayload[eventConfig.Name] = values
	f.mutex.Unlock()
	return f.redis.Publish(f.ctx, RedisChannelPrefix+eventConfig.Name+"."+action, json)
}
