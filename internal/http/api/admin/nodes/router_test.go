package nodes

import (
	"testing"

	"github.com/Ithildur/EiluneKit/http/routes"
)

func TestRouterKeepsAdminNodePaths(t *testing.T) {
	routeList := Router(nil, nil, nil).Routes()
	want := []struct {
		method string
		path   string
	}{
		{method: "PATCH", path: "/{id}"},
		{method: "DELETE", path: "/{id}"},
		{method: "POST", path: "/{id}/upgrade"},
		{method: "GET", path: "/traffic/rebuild"},
		{method: "POST", path: "/{id}/traffic/rebuild"},
	}

	for _, item := range want {
		if hasAdminNodeRoute(routeList, item.method, item.path) {
			continue
		}
		t.Fatalf("route %s %s missing; got %v", item.method, item.path, adminNodeRouteKeys(routeList))
	}
}

func hasAdminNodeRoute(routeList []routes.Route, method, path string) bool {
	for _, route := range routeList {
		if route.Method == method && route.Path == path {
			return true
		}
	}
	return false
}

func adminNodeRouteKeys(routeList []routes.Route) []string {
	out := make([]string, 0, len(routeList))
	for _, route := range routeList {
		out = append(out, route.Method+" "+route.Path)
	}
	return out
}
