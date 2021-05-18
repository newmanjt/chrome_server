package chrome_server

import (
	"encoding/json"
	"fmt"
	"github.com/newmanjt/common"
	"github.com/raff/godet"
	"os/exec"
	"time"
)

// GlobalRemote contains the reference to the Chrome Devtools Protocol connection
var GlobalRemote *godet.RemoteDebugger

type GlobalRequest struct {
	Type     string
	Tab      godet.Tab
	ID       string
	Url      string
	JS       string
	Path     string
	RespChan chan interface{}
}

type GlobalResponse struct {
	Tab     godet.Tab
	JS      interface{}
	Loc     string
	Res     PerformanceTiming
	Results interface{}
}

var RemoteChan chan GlobalRequest
var NewTabChan chan GlobalResponse
var EvaluateJSChan chan GlobalResponse

func RemoteServer() {
	GlobalRemote = GetRemote()
	GlobalRemote.Verbose = false
	defer GlobalRemote.Close()

	for {
		select {
		case req := <-RemoteChan:
			switch req.Type {
			case "new_tab":
				tab, err := GlobalRemote.NewTab(req.Url)
				if err != nil {
					fmt.Println(err.Error())
				}
				NewTabChan <- GlobalResponse{Tab: *tab}
			case "activate_tab":
				GlobalRemote.ActivateTab(&req.Tab)
			case "close_tab":
				fmt.Println(fmt.Sprintf("closing tab: %+v", req.Tab))
				GlobalRemote.CloseTab(&req.Tab)
			case "evaluate_js":
				res, err := GlobalRemote.EvaluateWrap(req.JS)
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
				EvaluateJSChan <- GlobalResponse{JS: res}
			case "search":
				tab, err := GlobalRemote.NewTab(req.Url)
				if err != nil {
					fmt.Println(err.Error())
				}
				time.Sleep(2 * time.Second)
				GlobalRemote.ActivateTab(tab)
				res, err := GlobalRemote.EvaluateWrap(req.JS)
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
				GlobalRemote.CloseTab(tab)
				req.RespChan <- res
			case "evaluate":
				GlobalRemote.ActivateTab(&req.Tab)
				res, err := GlobalRemote.EvaluateWrap(req.JS)
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
				var timing PerformanceTiming

				json.Unmarshal([]byte(res.(string)), &timing)
				err = GlobalRemote.SaveScreenshot(req.Path+"/data/screenshots/"+req.ID, 0644, 0, true)
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
				GlobalRemote.CloseTab(&req.Tab)
				req.RespChan <- JSEval{Loc: req.ID, Res: timing}
			case "screen_shot":
				err := GlobalRemote.SaveScreenshot(req.ID, 0644, 0, true)
				if err != nil {
					fmt.Println(err.Error())
					continue
				}
			case "clear_tabs":
				tabs, err := GlobalRemote.TabList("")
				fmt.Println(fmt.Sprintf("%+v", tabs))
				common.CheckError(err)
				for i, tab := range tabs {
					fmt.Println("closing tab " + string(i))
					if i == 0 {
						i += 1
						continue
					}
					GlobalRemote.CloseTab(tab)
				}
			}
		}
	}
}

func EvaluateJS(js string) (res interface{}) {
	RemoteChan <- GlobalRequest{JS: js, Type: "evaluate_js"}
	resp := <-EvaluateJSChan
	return resp.JS
}

func NewTab(url string) godet.Tab {
	RemoteChan <- GlobalRequest{Url: url, Type: "new_tab"}
	resp := <-NewTabChan
	return resp.Tab
}

func CloseTab(tab godet.Tab) {
	RemoteChan <- GlobalRequest{Tab: tab, Type: "close_tab"}
}

func ActivateTab(tab godet.Tab) {
	RemoteChan <- GlobalRequest{Tab: tab, Type: "activate_tab"}
}

func Search(url string, js string) interface{} {
	x := make(chan interface{})
	RemoteChan <- GlobalRequest{Url: url, Type: "search", JS: js, RespChan: x}
	select {
	case y := <-x:
		return y
	}
}

func Evaluate(url string, name string, user string, js string, x chan interface{}, thumb_size string, path string) {
	tab := NewTab(url)
	time.Sleep(2 * time.Second)
	RemoteChan <- GlobalRequest{ID: name, Type: "evaluate", Tab: tab, JS: js, RespChan: x, Path: path}
	for {
		cmd := exec.Command("sudo", "-u", user, "convert", "./data/screenshots/"+name, "-resize", thumb_size, path+"/data/screenshots/small_"+name)
		out, err := cmd.CombinedOutput()
		if err == nil {
			break
		} else {
			fmt.Println(string(out))
		}
		time.Sleep(time.Second * 5)
	}
}

func SaveScreenshot(name string) {
	RemoteChan <- GlobalRequest{ID: name, Type: "screen_shot"}
}

func ClearTabs() {
	RemoteChan <- GlobalRequest{Type: "clear_tabs"}
	time.Sleep(time.Second * 2)
}

//GetRemote establishes a connection to the |Browser| using Chrome Devtools Protocol
func GetRemote() (remote *godet.RemoteDebugger) {
	remote, err := godet.Connect("localhost:9222", true)
	common.CheckError(err)
	return
}

type PerformanceTiming struct {
	SpeedIndex                 float64 `json:"rum_si"`
	FirstPaint                 float64 `json:"rum_fp"`
	Images                     float64 `json:"rects"`
	Words                      float64 `json:"words"`
	Scripts                    float64 `json:"scripts"`
	ConnectEnd                 float64 `json:"connectEnd"`
	ConnectStart               float64 `json:"connectStart"`
	DomComplete                float64 `json:"domComplete"`
	DomContentLoadedEventEnd   float64 `json:"domContentLoadedEventEnd"`
	DomContentLoadedEventStart float64 `json:"domContentLoadedEventStart"`
	DomInteractive             float64 `json:"domInteractive"`
	DomLoading                 float64 `json:"domLoading"`
	DomainLookupEnd            float64 `json:"domainLookupEnd"`
	DomainLookupStart          float64 `json:"domainLookupStart"`
	FetchStart                 float64 `json:"fetchStart"`
	LoadEventEnd               float64 `json:"loadEventEnd"`
	LoadEventStart             float64 `json:"loadEventStart"`
	NavigationStart            float64 `json:"navigationStart"`
	RedirectEnd                float64 `json:"redirectEnd"`
	RedirectStart              float64 `json:"redirectStart"`
	RequestStart               float64 `json:"requestStart"`
}

type JSEval struct {
	Loc string
	Res PerformanceTiming
}
