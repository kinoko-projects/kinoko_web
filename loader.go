package kinoko_web

import "github.com/kinoko-projects/kinoko"

func init() {
	kinoko.Application.Use(new(HttpConfig), new(HttpServer), new(SQL), &sqlPropertiesHolder)
}
