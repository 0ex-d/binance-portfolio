package main

import (
	"fmt"
	"html/template"
	"io"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

type Template struct {
	tmpls *template.Template
}

func (t *Template) Render(w io.Writer, name string, data interface{}, c echo.Context) error {
	return t.tmpls.ExecuteTemplate(w, name, data)
}

func main() {
	e := echo.New()

	e.Renderer = &Template{
		tmpls: template.Must(template.ParseGlob("views/*.html")),
	}

	e.Use(middleware.Logger())
    e.Static("/src", "src")

	e.GET("/", func(c echo.Context) error {
		return c.Render(200, "index", nil)
	})
port:="42000"
	e.Logger.Fatal(e.Start(fmt.Sprintf(":%s",port)))
}
