package fetcher

import (
	"time"

	"github.com/google/uuid"
)

func (f *Fetcher) handleSession(values map[string]any, eventConfig EventConfig) error {

	changed := f.stateChanged(values, eventConfig)
	allowed := f.publishWhen(values, eventConfig.PublishWhen)

	if !allowed && changed {
		return f.endSessionAndPublish(eventConfig)
	}
	if !allowed || !changed {
		return nil
	}

	f.mutex.RLock()
	hasLastPayload := len(f.lastPayload[eventConfig.Name]) > 0
	f.mutex.RUnlock()
	if hasLastPayload {
		if err := f.endSessionAndPublish(eventConfig); err != nil {
			return err
		}
	}

	f.startSession(values)
	return f.publish(values, eventConfig, ActionCreated)
}

func (f *Fetcher) startSession(values map[string]any) {
	values[FieldID] = uuid.New().String()
	values[FieldStartedAt] = time.Now().Format(time.RFC3339)
}

func (f *Fetcher) endSession(name string) {
	f.mutex.Lock()
	if f.lastPayload[name] != nil {
		f.lastPayload[name][FieldEndedAt] = time.Now().Format(time.RFC3339)
	}
	f.mutex.Unlock()
}

func (f *Fetcher) endSessionAndPublish(eventConfig EventConfig) error {
	f.endSession(eventConfig.Name)
	f.mutex.RLock()
	lastPayload, exists := f.lastPayload[eventConfig.Name]
	f.mutex.RUnlock()
	if !exists || lastPayload == nil {
		return nil
	}
	return f.publish(lastPayload, eventConfig, ActionEnded)
}
