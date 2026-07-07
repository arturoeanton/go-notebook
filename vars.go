package main

import (
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/cosmos72/gomacro/fast"
)

type varInfo struct {
	Name  string
	Type  string
	Value string
}

// listVars enumerates the interpreter's top-level bindings, skipping the
// baseline names (preamble helpers, host functions, imported packages).
func listVars(interp *fast.Interp, baseline map[string]bool) []varInfo {
	c := interp.Comp
	env := interp.PrepareEnv()

	out := make([]varInfo, 0, 16)
	for name, bind := range c.Binds {
		if bind == nil || baseline[name] || strings.HasPrefix(name, "_") {
			continue
		}
		out = append(out, describeBind(c, env, name, bind))
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func describeBind(c *fast.Comp, env *fast.Env, name string, bind *fast.Bind) (info varInfo) {
	info.Name = name
	info.Type = "?"
	info.Value = "?"
	defer func() {
		recover() // some binds cannot be materialized; show what we have
	}()
	if bind.Type != nil {
		info.Type = bind.Type.String()
	}
	if strings.HasPrefix(info.Type, "func") {
		info.Value = "func"
		return info
	}
	v := bind.RuntimeValue(c.CompGlobals, env)
	if !v.IsValid() {
		info.Value = "nil"
		return info
	}
	s := fmt.Sprint(v.ReflectValue())
	s = strings.Join(strings.Fields(s), " ") // collapse newlines/whitespace
	if len(s) > 140 {
		s = s[:140] + "…"
	}
	info.Value = s
	return info
}

// baselineBinds snapshots the current binding names, used to hide the
// preamble helpers and host functions from the variables panel.
func baselineBinds(interp *fast.Interp) map[string]bool {
	out := make(map[string]bool, len(interp.Comp.Binds))
	for name := range interp.Comp.Binds {
		out[name] = true
	}
	return out
}

// refreshVars re-renders the variables panel for this session.
func (s *Session) refreshVars() {
	s.mu.Lock()
	page := s.page
	interp := s.interp
	baseline := s.varsBaseline
	s.mu.Unlock()
	if page == nil || interp == nil {
		return
	}

	vars := listVars(interp, baseline)
	var sb strings.Builder
	if len(vars) == 0 {
		sb.WriteString(`<div class="vars-empty">No variables yet — run a cell.</div>`)
	} else {
		sb.WriteString(`<table class="vars-table">`)
		for _, v := range vars {
			sb.WriteString("<tr><td class=\"vars-name\">" + html.EscapeString(v.Name) +
				"</td><td class=\"vars-type\">" + html.EscapeString(v.Type) +
				"</td><td class=\"vars-value\">" + html.EscapeString(v.Value) + "</td></tr>")
		}
		sb.WriteString("</table>")
	}
	page.FillValueById("vars_body", sb.String())
}
