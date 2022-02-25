package main

import (
	"reflect"
	"strings"
	"text/template"

	pgs "github.com/lyft/protoc-gen-star"
	pgsts "github.com/snikch/protoc-gen-star-lang-ts"
)

// XStatePlugin
type XStateModule struct {
	*pgs.ModuleBase
	ctx pgsts.Context
	tpl *template.Template
}

// XState returns an initialized XStatePlugin
func XState() *XStateModule { return &XStateModule{ModuleBase: &pgs.ModuleBase{}} }

func (p *XStateModule) InitContext(c pgs.BuildContext) {
	p.ModuleBase.InitContext(c)
	p.ctx = pgsts.InitContext(c.Parameters())

	tpl := template.New("XState").Funcs(map[string]interface{}{
		"eventtype":    p.eventType,
		"eventtypes":   p.eventTypes,
		"package":      p.ctx.PackageName,
		"name":         p.ctx.Name,
		"hasstream":    p.hasStream,
		"filename":     p.ctx.FileName,
		"join":         strings.Join,
		"requesttype":  p.requestType,
		"responsetype": p.responseType,
		"last": func(x int, a interface{}) bool {
			return x == reflect.ValueOf(a).Len()-1
		},
		"exceptlast": func(x int, a interface{}, str string) string {
			if x == reflect.ValueOf(a).Len()-1 {
				return ""
			}
			return str
		},
	})

	p.tpl = template.Must(tpl.Parse(XStateTpl))
}

// Name satisfies the generator.Plugin interface.
func (p *XStateModule) Name() string { return "xstate" }

func (p *XStateModule) Execute(targets map[string]pgs.File, pkgs map[string]pgs.Package) []pgs.Artifact {

	for _, t := range targets {
		p.generate(t)
	}

	return p.Artifacts()
}

func (p *XStateModule) generate(f pgs.File) {
	if len(f.Messages()) == 0 {
		return
	}

	if len(f.Services()) > 0 {
		name := p.ctx.OutputPath(f).SetExt(".xstate.ts")
		p.AddGeneratorTemplateFile(name.String(), p.tpl, f)
	}
}

func (p *XStateModule) eventType(m pgs.Method) pgs.Name {
	return "Event" + m.Service().Name() + m.Name()
}

func (p *XStateModule) requestType(m pgs.Method) pgs.Name {
	return m.Input().Name()
}

func (p *XStateModule) responseType(m pgs.Method) pgs.Name {
	return m.Output().Name()
}
func (p *XStateModule) ioTypes(m pgs.File) []string {
	out := make([]string, 0, 100)
	for _, service := range m.Services() {
		for _, method := range service.Methods() {
			out = append(out, m.Name().String())
			out = append(out, p.requestType(method).String())
			out = append(out, p.responseType(method).String())
		}
	}
	return out
}
func (p *XStateModule) eventTypes(m pgs.Node) []string {
	if file, ok := m.(pgs.File); ok {
		out := make([]string, 0, 100)
		for _, service := range file.Services() {
			for _, method := range service.Methods() {
				out = append(out, p.eventType(method).String())
			}
		}
		return out
	}
	if service, ok := m.(pgs.Service); ok {
		out := make([]string, 0, 100)
		for _, method := range service.Methods() {
			out = append(out, p.eventType(method).String())
		}
		return out
	}
	return []string{}
}

func (p *XStateModule) hasStream(m pgs.File) bool {
	for _, service := range m.Services() {
		for _, method := range service.Methods() {
			if method.ServerStreaming() {
				return true
			}
		}
	}
	return false
}

// func (p *XStateModule) unmarshaler(m pgs.Message) pgs.Name {
// 	return p.ctx.Name(m) + "JSONUnmarshaler"
// }

const XStateTpl = `
{{ $eventTypes := eventtypes .}}
import { grpc } from "@improbable-eng/grpc-web"
{{- if hasstream . }}
import { Observable } from "rxjs"{{ end }}
import {
  {{ range .Services -}}
  {{ .Name }},
  {{- range .Methods }}
  {{ requesttype . }},
  {{ responsetype . }},
  {{- end }}{{ end }}
} from "./{{ filename .File "" }}"

{{ $package := package . }}
{{- range .Services }}{{ range .Methods -}}
export interface {{ eventtype . }} {
    type: "{{ $package }}.{{ .Service.Descriptor.Name}}.{{ .Descriptor.Name}}",
    data: {{ .Input.Descriptor.Name }},
    metadata?: grpc.Metadata,
}
{{ end }}{{- end }}
{{- range .Services }}
export type Event{{ .Name }} =
{{ $names := eventtypes . }}{{ range $names }}  | {{ . }}
{{ end -}}
{{ end -}}
{{- $services := .Services }}
export type Event = {{ range $i, $e := .Services }}{{ $e.Name }}{{ exceptlast $i $services " | " }}{{ end }}
{{ range .Services }}
export interface {{ .Name }}StateChartContext {
  service: {{ .Name }}
}

export const {{ .Name }}StateChartServices = {
{{- range .Methods }}
  {{ .Name }}: <TContext>(
    ctx: TContext & {{ .Service.Name }}StateChartContext,
    ev: {{ eventtype . }},
  ): {{ if .ServerStreaming }}Observable{{ else }}Promise{{ end }}<{{ .Output.Name }}> => {
    return ctx.service.{{ .Name }}(ev.data, ev.metadata)
  },{{ end }}
}
{{ end }}
`
