package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
)

const (
	gladosUrl      = "https://glados.rocks/api/user/checkin"
	pushToken      = "xxx_push_token"
	v2freeLoginUrl = "https://v2free.org/auth/login"
	v2freeCheckUrl = "https://v2free.org/user/checkin"
)

type Account struct {
	ID       string
	Cookie   string
	Password string
	Req      Request
}

type Result struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Ret     int    `json:"ret"`
	Msg     string `json:"msg"`
}

var gladosAccounts = []Account{
	{
		ID:     "xxx@qq.com",
		Cookie: "xxx_cookie",
		Req: Request{
			URL:     gladosUrl,
			Method:  http.MethodPost,
			Payload: strings.NewReader(`{"token":"glados.network"}`),
			Headers: map[string]string{"content-type": "application/json;charset=UTF-8"},
		},
	},
}

var v2freeAccounts = []Account{
	{
		ID:       "xx@qq.com",
		Password: "pwd",
		Req: Request{
			URL:    v2freeLoginUrl,
			Method: http.MethodPost,
			Headers: map[string]string{
				"content-type": "application/x-www-form-urlencoded; charset=UTF-8",
				"referer":      "https://v2free.org/auth/login",
			},
		},
	},
}

func (acc *Account) getCookie(cookies []string) string {
	var sb strings.Builder
	for _, v := range cookies {
		parts := strings.Split(v, ";")
		uid := strings.TrimSpace(parts[0])
		sb.WriteString(uid)
		sb.WriteString("; ")
	}

	str := sb.String()
	str = strings.TrimRight(str, "; ")

	return str
}

type HTTPRequester interface {
	SendRequest() (*http.Response, error)
	GetResponseBody(res *http.Response) ([]byte, error)
	GetResponseFunc() (*http.Response, []byte, error)
}

var _ HTTPRequester // 确保实现了 HTTPRequester接口

type Request struct {
	URL     string
	Method  string
	Payload io.Reader
	Headers map[string]string
}

func MapToJson(param map[string]string) string {
	dataType, _ := json.Marshal(param)
	dataString := string(dataType)
	return dataString
}

func (req *Request) sendRequest() (*http.Response, error) {
	client := &http.Client{}

	httpReq, err := http.NewRequest(req.Method, req.URL, req.Payload)
	if err != nil {
		return nil, err
	}

	for key, value := range req.Headers {
		httpReq.Header.Add(key, value)
	}

	res, err := client.Do(httpReq)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func (req *Request) SendRequest() (*http.Response, error) {
	var res *http.Response
	var err error

	// 设置请求超时时间为 5 秒钟
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 使用 for 循环进行超时重试
	for i := 0; i < 3; i++ {
		// 发送请求
		res, err = req.sendRequest()
		if err == nil {
			break
		}

		// 如果请求失败，则等待一段时间后再次尝试
		fmt.Printf("Request failed on attempt %d: %v\n", i+1, err)
		select {
		case <-ctx.Done():
			// 超时或主动取消
			return nil, fmt.Errorf("Request timed out: %v", ctx.Err())
		case <-time.After(2 * time.Second):
			// 等待 2 秒钟后再次尝试
		}
	}

	if err != nil {
		return nil, err
	}

	return res, nil
}

func (req *Request) GetResponseBody(res *http.Response) ([]byte, error) {
	body, err := io.ReadAll(res.Body)
	defer res.Body.Close()

	if err != nil {
		return nil, err
	}

	return body, nil
}

func (req *Request) GetAccountResponseFunc(acc *Account) (*http.Response, []byte, error) {
	req.Headers["cookie"] = acc.Cookie // 装饰器模式，对 GetResponseFunc 进行装饰增加设置cookie的设置以重用HTTPRequest接口

	return req.GetResponseFunc()
}

func (req *Request) GetResponseFunc() (*http.Response, []byte, error) {
	res, err := req.SendRequest()

	if err != nil {
		return nil, nil, err
	}

	body, err := req.GetResponseBody(res)

	if err != nil {
		return nil, nil, err
	}

	return res, body, nil
}

type pushMap map[string]string

func (push pushMap) CheckinGla() {
	wg := sync.WaitGroup{}

	for _, account := range gladosAccounts {
		wg.Add(1)
		_, body, err := account.Req.GetAccountResponseFunc(&account)
		if err != nil {
			push[account.ID] = err.Error()
			continue
		}

		result := Result{}
		_ = json.Unmarshal(body, &result)
		fmt.Println(account.ID, ":", result.Message)
		push[account.ID] = result.Message
		wg.Done()
	}
	wg.Wait()
}

func (push pushMap) CheckinV2f() {
	wg := sync.WaitGroup{}

	for _, account := range v2freeAccounts {
		wg.Add(1)
		go func(acc Account) {

			acc.Req.Payload = strings.NewReader(fmt.Sprintf("email=%s&passwd=%s&code=", url.QueryEscape(acc.ID), url.QueryEscape(acc.Password)))

			res, err := acc.Req.SendRequest()
			if err != nil {
				push[acc.ID] = err.Error()
				return
			}

			acc.Req.URL = v2freeCheckUrl
			acc.Req.Payload = strings.NewReader("")
			acc.Req.Headers["referer"] = "https://v2free.org/user"
			cookies := res.Header["Set-Cookie"]
			acc.Cookie = acc.getCookie(cookies)

			_, body, err := acc.Req.GetAccountResponseFunc(&acc)

			if err != nil {
				push[acc.ID] = err.Error()
				return
			}

			result := Result{}
			err = json.Unmarshal(body, &result)
			if err != nil {
				push[acc.ID] = err.Error()
				return
			}
			fmt.Println(acc.ID, ":", result.Msg)
			push[acc.ID] = result.Msg
			wg.Done()
		}(account)
	}
	wg.Wait()
}

func main() {
	push := pushMap{}
	push.CheckinGla()
	push.CheckinV2f()

	req := Request{
		URL:     fmt.Sprintf("http://www.pushplus.plus/send?token=%s&content=%s&title=GlaDOS%s", pushToken, url.QueryEscape(MapToJson(push)), url.QueryEscape("GlaDOS签到")),
		Method:  http.MethodGet,
		Payload: nil,
		Headers: nil,
	}

	_, _ = req.SendRequest()
}
