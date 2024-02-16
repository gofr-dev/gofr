package migration

type usageTracker interface {
	set()
	get() bool
}

type usage struct {
	status bool
}

func (r *usage) set() {
	r.status = true
}

func (r *usage) get() bool {
	return r.status
}
