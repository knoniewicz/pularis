package manager

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/knoniewicz/pularis/config"
	"github.com/knoniewicz/pularis/fetcher"
	"github.com/knoniewicz/pularis/internal/httpclient"
	"github.com/knoniewicz/pularis/internal/redispub"

	"github.com/BurntSushi/toml"
	"github.com/fsnotify/fsnotify"
)

type Manager struct {
	ctx             context.Context
	wg              sync.WaitGroup
	client          *httpclient.Client
	workers         int
	fetchers        []*fetcher.Fetcher
	fetcherWorkpool chan *fetcher.Fetcher
	authTokens      map[string]map[string]any
	configs         []fetcher.Config
}

func (m *Manager) initWorkers(ctx context.Context) {
	for i := 0; i < m.workers; i++ {
		m.wg.Add(1)
		go m.workerLoop(ctx)
	}
}

func (m *Manager) workerLoop(ctx context.Context) {
	defer m.wg.Done()
	for {
		select {
		case <-ctx.Done():
			return
		case f := <-m.fetcherWorkpool:
			if f == nil {
				continue
			}
			if err := f.Fetch(); err != nil {
				slog.Error("error fetching source", "fetcher", f.Name, "error", err)
			}

			go func(fetcher *fetcher.Fetcher) {
				ticker := time.NewTicker(time.Duration(fetcher.Cfg.PollIntervalSeconds) * time.Second)
				defer ticker.Stop()

				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					select {
					case m.fetcherWorkpool <- fetcher:
					case <-ctx.Done():
						return
					}
				}
			}(f)
		}
	}
}

func NewManager(ctx context.Context, redisClient *redispub.Client, pularisCfg config.Config) *Manager {
	m := &Manager{
		ctx:             ctx,
		wg:              sync.WaitGroup{},
		client:          httpclient.New(),
		workers:         pularisCfg.MaxWorkers,
		fetcherWorkpool: make(chan *fetcher.Fetcher, pularisCfg.MaxWorkers),
		fetchers:        make([]*fetcher.Fetcher, 0),
		authTokens:      make(map[string]map[string]any),
		configs:         make([]fetcher.Config, 0),
	}

	err := filepath.Walk(pularisCfg.ConfigsDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		if !strings.HasSuffix(path, ".toml") && !strings.HasSuffix(path, fetcher.AuthFileSuffix) {
			return nil
		}

		err = m.loadAuthTokens(path)
		if err != nil {
			slog.Error("not a valid auth file", "path", path, "error", err)
		}
		if err := m.watchAuthFile(path); err != nil {
			slog.Error("failed to watch auth file", "path", path, "error", err)
		}

		err = m.loadFetcherConfigs(path)
		if err != nil {
			slog.Error("failed to load fetcher configs", "path", path, "error", err)
		}

		return nil
	})

	if err != nil && !errors.Is(err, os.ErrNotExist) {
		slog.Error("failed to walk configs", "error", err)
	} else if errors.Is(err, os.ErrNotExist) {
		slog.Warn("configs directory not found, starting without sources", "path", pularisCfg.ConfigsDir)
	}

	for i := range m.configs {
		if err := m.createFetcher(ctx, redisClient, m.configs[i], pularisCfg); err != nil {
			slog.Error("failed to create fetcher", "config", m.configs[i].Name, "error", err)
		}
	}

	return m
}

func (m *Manager) loadFetcherConfigs(path string) error {
	if !strings.HasSuffix(path, ".toml") {
		return nil
	}

	var fetcherCfg *fetcher.Config
	_, err := toml.DecodeFile(path, &fetcherCfg)
	if err != nil {
		return fmt.Errorf("failed to decode fetcher config: %v", err)
	}

	fetcherCfg.Endpoint = os.ExpandEnv(fetcherCfg.Endpoint)
	if fetcherCfg.Headers != nil {
		for k, v := range fetcherCfg.Headers {
			fetcherCfg.Headers[k] = os.ExpandEnv(v)
		}
	}

	m.configs = append(m.configs, *fetcherCfg)
	return nil
}

func (m *Manager) createFetcher(ctx context.Context, redisClient *redispub.Client, fetcherCfg fetcher.Config, pularisCfg config.Config) error {
	f := fetcher.NewFetcher(ctx, m.authTokens[fetcherCfg.AuthFile], m.client, redisClient, fetcherCfg, pularisCfg)
	m.fetchers = append(m.fetchers, f)
	return nil
}

func (m *Manager) watchAuthFile(path string) error {
	if !strings.HasSuffix(path, fetcher.AuthFileSuffix) {
		return nil
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		slog.Error("failed to create fsnotify watcher", "error", err, "path", path)
		return err
	}

	if err := watcher.Add(path); err != nil {
		watcher.Close()
		return fmt.Errorf("failed to add path to watcher: %w", err)
	}

	authKey := strings.TrimSuffix(filepath.Base(path), fetcher.AuthFileSuffix)

	go func() {
		defer watcher.Close()
		for {
			select {
			case <-m.ctx.Done():
				return
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					slog.Info("auth file changed, reloading", "path", path)
					if err := m.loadAuthTokens(path); err != nil {
						slog.Error("failed to reload auth tokens", "path", path, "error", err)
						continue
					}
					m.refreshFetcherAuth(authKey)
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				slog.Error("fsnotify watcher error", "error", err)
			}
		}
	}()
	return nil
}

func (m *Manager) loadAuthTokens(path string) error {
	if !strings.HasSuffix(path, fetcher.AuthFileSuffix) {
		return nil
	}
	tokenFile := strings.TrimSuffix(filepath.Base(path), fetcher.AuthFileSuffix)

	token, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read token file: %w", err)
	}

	var auth map[string]any
	err = json.Unmarshal(token, &auth)
	if err != nil {
		return fmt.Errorf("failed to unmarshal token: %w", err)
	}

	m.authTokens[tokenFile] = auth
	return nil
}

func (m *Manager) refreshFetcherAuth(authKey string) {
	auth := m.authTokens[authKey]
	for _, f := range m.fetchers {
		if f.Cfg.AuthFile == authKey {
			f.RefreshAuthHeaders(auth)
			slog.Info("refreshed auth headers for fetcher", "fetcher", f.Name)
		}
	}
}

func (m *Manager) Start() {
	m.initWorkers(m.ctx)
	for _, fetcher := range m.fetchers {
		m.fetcherWorkpool <- fetcher
	}
}

func (m *Manager) Close() {
	m.wg.Wait()
}
