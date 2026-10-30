// Package all registers every built-in plugin. Only pigeon's main imports
// it: core never imports a plugin, and a plugin never imports another.
package all

import (
	_ "github.com/hoki2020/pigeon/plugins/budget"
	_ "github.com/hoki2020/pigeon/plugins/gateway"
	_ "github.com/hoki2020/pigeon/plugins/groups"
	_ "github.com/hoki2020/pigeon/plugins/library"
	_ "github.com/hoki2020/pigeon/plugins/middleware"
	_ "github.com/hoki2020/pigeon/plugins/otel"
	_ "github.com/hoki2020/pigeon/plugins/redact"
	_ "github.com/hoki2020/pigeon/plugins/rotation"
	_ "github.com/hoki2020/pigeon/plugins/routing"
	_ "github.com/hoki2020/pigeon/plugins/search"
	_ "github.com/hoki2020/pigeon/plugins/sessions"
	_ "github.com/hoki2020/pigeon/plugins/tools"
	_ "github.com/hoki2020/pigeon/plugins/usage"
	_ "github.com/hoki2020/pigeon/plugins/warmups"
)
