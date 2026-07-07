package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/arturoeanton/go-echo-live-view/liveview"
	"github.com/gorilla/websocket"
	"github.com/labstack/echo/v4"
)

// registerLivePage serves the notebook page and its WebSocket endpoint. It
// replaces liveview.PageControl so the factory receives the authenticated
// user of each connection (PageControl's factory takes no request context).
func registerLivePage(e *echo.Echo, path string, store *AuthStore, factory func(user string) liveview.LiveDriver) {
	headCode, _ := liveview.FileToString("lives/head.html")
	css, _ := liveview.FileToString("lives/style.css")
	version := time.Now().Unix()

	pageHTML := `<html lang="en">
	<head>
		<title>Go Notebook</title>
		` + headCode + `
		<style>
` + css + `
		</style>
		<meta charset="utf-8"/>
		<script src="/assets/live.js?v=` + fmt.Sprint(version) + `"></script>
	</head>
	<body>
		<div id="content"></div>
	</body>
</html>`

	e.Static("/assets", "assets")
	e.GET(path, func(c echo.Context) error {
		return c.HTML(http.StatusOK, pageHTML)
	})

	e.GET(path+"ws_goliveview", func(c echo.Context) error {
		user := ""
		if store != nil {
			u, ok := userFromRequest(c, store)
			if !ok {
				return c.NoContent(http.StatusUnauthorized)
			}
			user = u
		}

		content := factory(user)
		if content == nil {
			return fmt.Errorf("factory returned nil")
		}
		defer func() {
			func() {
				liveview.MuLayout.Lock()
				defer liveview.MuLayout.Unlock()
				delete(liveview.Layouts, content.GetIDComponet())
			}()
			func() {
				defer liveview.HandleReoverPass()
				layout, ok := content.GetComponet().(*liveview.Layout)
				if !ok || layout.HandlerEventDestroy == nil {
					return
				}
				(*layout.HandlerEventDestroy)(content.GetIDComponet())
			}()
		}()

		content.SetID("content")

		channel := make(chan map[string]interface{}, 10)
		defer close(channel)

		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 1024,
		}
		ws, err := upgrader.Upgrade(c.Response(), c.Request(), nil)
		if err != nil {
			return err
		}
		defer ws.Close()
		ws.SetReadLimit(liveview.MaxMessageSize)

		rateLimiter := liveview.NewRateLimiter(1000, 1)
		clientID := c.RealIP()

		drivers := make(map[string]liveview.LiveDriver)
		channelIn := make(map[string]chan interface{})

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		go func() {
			defer liveview.HandleReover()
			content.StartDriverWithContext(ctx, &drivers, &channelIn, channel)
		}()

		done := make(chan struct{})
		defer close(done)
		go func() {
			defer liveview.HandleReover()
			for {
				select {
				case data, ok := <-channel:
					if !ok {
						return
					}
					ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
					if err := ws.WriteJSON(data); err != nil {
						return
					}
				case <-ctx.Done():
					return
				case <-done:
					return
				}
			}
		}()

		ws.SetReadDeadline(time.Now().Add(60 * time.Second))
		ws.SetPongHandler(func(string) error {
			ws.SetReadDeadline(time.Now().Add(60 * time.Second))
			return nil
		})

		// keepalive: without pings the read deadline drops idle sessions
		go func() {
			ticker := time.NewTicker(30 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ticker.C:
					ws.SetWriteDeadline(time.Now().Add(10 * time.Second))
					if err := ws.WriteMessage(websocket.PingMessage, nil); err != nil {
						return
					}
				case <-ctx.Done():
					return
				}
			}
		}()

		for {
			_, msg, err := ws.ReadMessage()
			if err != nil {
				return nil
			}
			ws.SetReadDeadline(time.Now().Add(60 * time.Second))

			if !rateLimiter.Allow(clientID, time.Now().Unix()) {
				continue
			}
			validatedMsg, err := liveview.ValidateWebSocketMessage(msg)
			if err != nil {
				continue
			}

			switch validatedMsg.Type {
			case "data":
				if driver, ok := drivers[validatedMsg.ID]; ok {
					driver.ExecuteEvent(validatedMsg.Event, validatedMsg.Data)
				}
			case "get":
				if ch, ok := channelIn[validatedMsg.IdRet]; ok {
					select {
					case ch <- validatedMsg.Data:
					case <-time.After(5 * time.Second):
					}
				}
			}
		}
	})
}
