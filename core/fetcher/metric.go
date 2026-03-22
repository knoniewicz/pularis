package fetcher

func (f *Fetcher) handleMetric(values map[string]interface{}, eventConfig EventConfig) error {
	if !f.publishWhen(values, eventConfig.PublishWhen) {
		return nil
	}
	return f.publish(values, eventConfig, ActionCreated)
}
