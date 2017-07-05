package ihui

import (
	"bytes"
	"fmt"

	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/websocket"
)

type Page struct {
	Renderer
	ws      *websocket.Conn
	buffer  bytes.Buffer
	title   string
	countID int
	actions map[string][]ActionFunc
}

func NewPage(session *Session, title string, render Renderer) *Page {
	page := &Page{
		ws:       session.ws,
		Renderer: render,
		title:    title,
	}
	return page
}

func (p *Page) Title() string {
	return p.title
}

func (p *Page) SetTitle(title string) {
	p.title = title
}

func (p *Page) Add(r Renderer) {
	r.Render(p)
}

func (p *Page) WriteString(html string) {
	p.buffer.WriteString(html)
}

func (p *Page) Write(data []byte) {
	p.buffer.Write(data)
}

func (p *Page) NewId() string {
	p.countID++
	return fmt.Sprintf("i%d", p.countID)
}

func (p *Page) On(id string, name string, action ActionFunc) {
	if action == nil {
		return
	}
	name = id + "." + name
	p.actions[name] = append(p.actions[name], action)
}

func (p *Page) Trigger(id string, name string, session *Session) {
	name = id + "." + name
	actions := p.actions[name]
	for _, action := range actions {
		action(session)
	}
}

func (page *Page) show(modal bool) (*Event, error) {
	page.actions = make(map[string][]ActionFunc)
	page.countID = 0

	page.buffer.Reset()
	page.buffer.WriteString(`<div id="main">`)
	page.Render(page)
	page.buffer.WriteString(`</div>`)

	doc, err := goquery.NewDocumentFromReader(&page.buffer)
	if err != nil {
		return nil, err
	}

	doc.Find("[data-action]").Each(func(i int, s *goquery.Selection) {
		_, ok := s.Attr("id")
		if !ok {
			return
		}

		action, _ := s.Attr("data-action")
		switch action {
		case "click":
			s.SetAttr("onclick", `sendMsg("click", $(this).attr("id"), null)`)

		case "check":
			s.SetAttr("onchange", `sendMsg("check", $(this).attr("id"), $(this).prop("checked"))`)

		case "change":
			s.SetAttr("onchange", `sendMsg("change", $(this).attr("id"), $(this).val())`)

		case "input":
			s.SetAttr("oninput", `sendMsg("change", $(this).attr("id"), $(this).val())`)

		case "submit":
			s.SetAttr("onsubmit", `sendMsg("form", $(this).attr("id"), $(this).serializeObject())`)

		case "form":
			s.Find("input[name], textarea[name], select[name]").Each(func(i int, ss *goquery.Selection) {
				ss.SetAttr("onchange", `sendMsg("change", $(this).attr("id"), { name: $(this).attr("name"), val: $(this).val() })`)
			})
		}

		s.RemoveAttr("data-action")
	})

	html, err := doc.Html()
	if err != nil {
		return nil, err
	}

	event := &Event{
		Name:   "update",
		Source: "main",
		Data: map[string]interface{}{
			"title": page.Title(),
			"html":  html,
		},
	}

	if err := page.sendEvent(event); err != nil {
		return nil, err
	}

	if err := websocket.ReadJSON(page.ws, event); err != nil {
		return nil, err
	}

	return event, nil
}

func (page *Page) sendEvent(event *Event) error {
	if err := websocket.WriteJSON(page.ws, event); err != nil {
		return err
	}
	return nil
}
