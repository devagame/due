package consul

import (
	"fmt"
	"strings"

	"github.com/devagame/due/v2/registry"
	"github.com/devagame/due/v2/utils/xconv"
)

const metaValueSize = 512

const (
	metaRouteInternal = 1 << iota
	metaRouteStateful
	metaRouteAuthorized
)

// 编码元数据路由
func marshalMetaRoutes(routes []registry.Route) map[string]string {
	var (
		key   string
		size  int
		metas = make(map[string]string)
		items string
	)

	for _, route := range routes {
		var opts int

		if route.Internal {
			opts |= metaRouteInternal
		}

		if route.Stateful {
			opts |= metaRouteStateful
		}

		if route.Authorized {
			opts |= metaRouteAuthorized
		}

		val := fmt.Sprintf("%d-%d", route.ID, opts)

		if s := len(items); s == 0 {
			size = len(val)
		} else {
			size = s + 1 + len(val)
		}

		if size <= metaValueSize {
			if len(items) == 0 {
				items = val
			} else {
				items += "," + val
			}
		}

		if size >= metaValueSize {
			key = fmt.Sprintf("%s-%d", metaFieldRoutes, len(metas))
			metas[key] = items
		}

		switch {
		case size < metaValueSize:
			// ignore
		case size > metaValueSize:
			items = val
		default:
			items = ""
		}
	}

	if len(items) > 0 {
		key = fmt.Sprintf("%s-%d", metaFieldRoutes, len(metas))
		metas[key] = items
	}

	return metas
}

// 解码元数据路由
func unmarshalMetaRoutes(metas map[string]string) []registry.Route {
	routes := make([]registry.Route, 0)

	for field, items := range metas {
		parts := strings.Split(field, "-")

		if len(parts) != 2 || parts[0] != metaFieldRoutes {
			continue
		}

		for _, item := range strings.Split(items, ",") {
			val := strings.Split(item, "-")

			if len(val) != 2 {
				continue
			}

			opts := xconv.Int(val[1])

			routes = append(routes, registry.Route{
				ID:         xconv.Int32(val[0]),
				Internal:   opts&metaRouteInternal != 0,
				Stateful:   opts&metaRouteStateful != 0,
				Authorized: opts&metaRouteAuthorized != 0,
			})
		}
	}

	return routes
}
