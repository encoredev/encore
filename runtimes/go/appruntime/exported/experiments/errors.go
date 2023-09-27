package experiments

// UnknownExperimentError is an error returned when an app tries to use
// an experiment that is not known to the current version of Encore.
type UnknownExperimentError struct {
	Name Name
}

func (e *UnknownExperimentError) Error() string {
	return "unknown experiment: " + string(e.Name)
}
