package trace

func (id *TraceID) IsZero() bool {
	return id == nil || (id.Low == 0 && id.High == 0)
}
