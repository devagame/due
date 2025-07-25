package discovery

import (
	"github.com/devagame/due/v2/errors"
	"github.com/devagame/due/v2/log"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/resolver"
)

type Resolver struct {
	builder *Builder
	target  resolver.Target
	cc      resolver.ClientConn
}

func (r *Resolver) ResolveNow(_ resolver.ResolveNowOptions) {
	r.builder.updateResolver(r)
}

func (r *Resolver) Close() {
	r.builder.removeResolver(r)
}

func (r *Resolver) updateState(state resolver.State) {
	if err := r.cc.UpdateState(state); err != nil {
		r.cc.ReportError(err)

		if !(len(state.Addresses) == 0 && errors.Is(err, balancer.ErrBadResolverState)) {
			log.Warnf("update client conn state failed: %v", err)
		}
	}
}
