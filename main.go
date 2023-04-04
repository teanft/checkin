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

var gladosInfo = map[string]any{
	"usr@usr.com":    "cookie",
}

var v2freeInfo = map[string]any{
	"user@user.com":         "password",
}

var pushToken = "XXX"

type Result struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Ret     int    `json:"ret"`
	Msg     string `json:"msg"`
}

func MapToJson(param map[string]any) string {
	dataType, _ := json.Marshal(param)
	dataString := string(dataType)
	return dataString
}

func sendHTTPRequest(url, method string, headers map[string]string, body io.Reader) (*http.Response, error) {
	client := &http.Client{}
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	return res, nil
}

func getResponseBody(url, method string, headers map[string]string, body io.Reader) ([]byte, error) {
	res, err := sendHTTPRequest(url, method, headers, body)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resBody, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("Request failed with status code: %v. Response body: %v", res.StatusCode, string(resBody))
	}

	return resBody, nil
}

func main() {
	push := make(map[string]any, len(gladosInfo)+len(v2freeInfo))

	var wg sync.WaitGroup
	for id, cookie := range gladosInfo {
		wg.Add(1)
		go func(id string, cookie any) {
			defer wg.Done()

			url := "https://glados.rocks/api/user/checkin"
			method := http.MethodPost
			headers := map[string]string{
				"Content-Type": "application/json;charset=UTF-8",
				"Cookie":       cookie.(string),
			}
			body := strings.NewReader(`{"token":"glados.network"}`)

			resBody, err := getResponseBody(url, method, headers, body)
			if err != nil {
				fmt.Println(err)
				return
			}
			result := Result{}
			json.Unmarshal(resBody, &result)
			push[id] = result.Message
			fmt.Println(id, ": ", result.Message)
		}(id, cookie)
	}
	wg.Wait()

	for id, pw := range v2freeInfo {
		wg.Add(1)
		go func(id string, pw any) {
			defer wg.Done()

			v2freeurl := "https://v2free.org/auth/login"
			method := http.MethodPost

			payload := strings.NewReader(fmt.Sprintf("email=%s&passwd=%s&code=", url.QueryEscape(id), url.QueryEscape(pw.(string))))
			headers := map[string]string{
				"Content-Type": "application/x-www-form-urlencoded; charset=UTF-8",
				"referer":      "https://v2free.org/auth/login",
			}

			req, err := sendHTTPRequest(v2freeurl, method, headers, payload)
			if err != nil {
				fmt.Println(err)
				return
			}

			defer func(Body io.ReadCloser) {
				err := Body.Close()
				if err != nil {
					fmt.Println(err)
					return
				}
			}(req.Body)

			resBody, err := io.ReadAll(req.Body)
			if err != nil {
				fmt.Println(err)
				return
			}

			if req.StatusCode >= 400 {
				fmt.Printf("Request failed with status code: %v. Response body: %v\n", req.StatusCode, string(resBody))
				return
			}

			v2freeurl = "https://v2free.org/user/checkin"
			payload = strings.NewReader("")
			headers["referer"] = "https://v2free.org/user"
			cookies := req.Header["Set-Cookie"]

			var sb strings.Builder
			for _, v := range cookies {
				parts := strings.Split(v, ";")
				uid := strings.TrimSpace(parts[0])
				sb.WriteString(uid)
				sb.WriteString("; ")
			}

			str := sb.String()
			str = strings.TrimRight(str, "; ")

			headers["cookie"] = str

			res, err := getResponseBody(v2freeurl, method, headers, payload)

			result := Result{}
			err = json.Unmarshal(res, &result)
			if err != nil {
				fmt.Println(err)
				return
			}

			push[id] = result.Msg
			fmt.Println(id, ": ", result.Msg)
		}(id, pw)
	}
	wg.Wait()

	getResponseBody(fmt.Sprintf("http://www.pushplus.plus/send?token=%s&content=%s&title=GlaDOS%s", pushToken, url.QueryEscape(MapToJson(push)), url.QueryEscape("GlaDOS签到")), http.MethodGet, nil, strings.NewReader(""))
}
