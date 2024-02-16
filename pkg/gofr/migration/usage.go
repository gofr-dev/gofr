package migration

type usageTracker interface {
	setUsage()
	checkUsage() bool
}

type usage struct {
	status bool
}

func (r *usage) setUsage() {
	r.status = true
}

func (r *usage) checkUsage() bool {
	return r.status
}
