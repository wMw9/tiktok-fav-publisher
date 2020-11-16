package main

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"encoding/json"
	"mime/multipart"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"github.com/tidwall/gjson"
)

const (
	makabaUrl  = "https://2ch.hk/makaba/makaba.fcgi"
	postingUrl = "https://2ch.hk/makaba/posting.fcgi?json=1"
)

type Passcode struct {
	Usercode string
	Error    bool
}

var CurrentUsercode Passcode = Passcode{
	Usercode: "",
	Error:    false,
}

func customClient() (*http.Client, bool) {
	jar, _ := cookiejar.New(nil)
	var cookies []*http.Cookie
	auth := CurrentUsercode.PasscodeAuth()
	if auth == false {
		log.Println("Failed to authorize passcode. Skip.")
	}
	cookie := &http.Cookie{
		Name:   "passcode_auth",
		Value:  CurrentUsercode.Usercode,
		Path:   "/",
		Domain: "2ch.hk",
	}
	cookies = append(cookies, cookie)
	u, _ := url.Parse(postingUrl)
	jar.SetCookies(u, cookies)
	//log.Println(jar.Cookies(u))
	client := &http.Client{
		Jar: jar,
	}
	return client, auth
}

// PasscodeAuth is used to authorize your passcode to get usercode. Used to bypass captcha
func (c *Passcode) PasscodeAuth() bool {
	formData := url.Values{
		"json":     {"1"},
		"task":     {"auth"},
		"usercode": {cfg.Passcode}}
	resp, err := http.PostForm(makabaUrl, formData)
	if err != nil {
		log.Println(err)
		return false
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println(err)
		return false
	}
	//log.Println(string(body))

	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if err != nil {
		log.Println(err)
		return false
	}
	//log.Println(result)
	if result["result"].(float64) == 0 {
		log.Println(result["description"])
		return false
	}
	if result["result"].(float64) == 1 {
		hash := fmt.Sprint(result["hash"])
		log.Println("✅ Got passcode_auth:", result["hash"])
		c.Usercode = hash
		c.Error = false
		return true
	}

	return false
}

func repost2ch(url string) bool {
	board, thread := findThread()
	log.Printf("https://2ch.hk/%v/res/%v.html", board, thread)
	valuesBase := prepareBase(board, thread)
	valuesFiles := prepareFiles(url)

	client, ok := customClient()
	if ok == false {
		return false
	}
	//fmt.Println("valuesFiles type is:", reflect.TypeOf(valuesFiles))
	err, success, num := makabaPost(client, postingUrl, valuesBase, valuesFiles)
	if err != nil {
		log.Println(err)
	}
	if success {
		log.Printf("%v", num)
	}
	return success
}

func makabaPost(client *http.Client, url string, valuesBase map[string]io.Reader, valuesFiles map[string]io.Reader) (err error, success bool, num float64) {
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	for key, r := range valuesBase {
		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		if fw, err = w.CreateFormField(key); err != nil {
			return
		}
		if _, err = io.Copy(fw, r); err != nil {
			return err, false, num
		}

	}
	for key, r := range valuesFiles {
		var fw io.Writer
		if x, ok := r.(io.Closer); ok {
			defer x.Close()
		}
		// Add an image file
		if fw, err = w.CreateFormFile(key, ""); err != nil {
			return
		}
		if _, err = io.Copy(fw, r); err != nil {
			return err, false, num
		}

	}
	w.Close()

	// Prepare handler.
	req, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return
	}
	req.Header.Set("Content-Type", w.FormDataContentType())

	// Высрать в тред
	res, err := client.Do(req)
	if err != nil {
		log.Println("client.Do(req) error:", err)
		return
	}
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println("ioutil.ReadAll error:", err)
		return
	}
	var result map[string]interface{}
	err = json.Unmarshal(body, &result)
	if result["Error"] != nil {
		log.Println("Makaba post error:", result)
	}
	log.Println(result)
	if result["Error"] == nil {
		log.Println("Successfully made post 👌🏻")
		success = true
		num = result["Num"].(float64)
		log.Printf("%v", result["Num"])
	}
	// Check the response
	if res.StatusCode != http.StatusOK {
		err = fmt.Errorf("bad status: %s", res.Status)
	}
	return err, success, num
}

func getCatalog() ([]byte, string, string) {
	board := "fag"
	keyword := "самых ламповых"
	//keyword := "навальный \\ролл /ролл"
	url := fmt.Sprintf("https://2ch.hk/%v/threads.json", board)
	resp, err := http.Get(url)
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	//fmt.Println(string(body))
	return body, keyword, board
}

func findThread() (string, string) {
	catalogJson, keyword, board := getCatalog()
	threads := gjson.GetBytes(catalogJson, `threads.#.subject`)
	var ind int // Thread index
	for k, v := range threads.Array() {
		if strings.Contains(strings.ToLower(v.String()), keyword) == true {
			fmt.Println("Thread found; Index is:", k, "; subject is:", v)
			ind = k
		}
	}
	gjsonPath := fmt.Sprintf("threads.%v.num", ind)

	num := gjson.GetBytes(catalogJson, gjsonPath).String()
	//fmt.Println("Thread number is:", num)
	return board, num
}

func prepareBase(board string, thread string) map[string]io.Reader {
	var baseReader map[string]io.Reader
	var comment string
	var name string
	comment = ""
	name = fmt.Sprintf("габи лайкнула тикток")
	//comment = fmt.Sprintf("[sup]Стрим запустился! %v ⛓[/sup]\n\n", jsonPayload.Source)

	baseReader = map[string]io.Reader{
		"task": strings.NewReader("post"),
		//"board":  strings.NewReader(json["2ch_board"].(string)),  // https://2ch.hk/test/
		"board":  strings.NewReader(board),  // https://2ch.hk/test/
		"thread": strings.NewReader(thread), // https://2ch.hk/test/res/28394.html
		"name":   strings.NewReader(name),   // Tripcode for attention whore
		//"email": strings.NewReader(""), // R u fucking kidding me?
		//"subject": strings.NewReader(jsonPayload.Person), // Oldfags never use it
		"comment": strings.NewReader(comment), // Post text

		//"comment": strings.NewReader(caption), // Post text
	}
	return baseReader
}

func prepareFiles(url string) map[string]io.Reader {
	var filesReader map[string]io.Reader

	// I know, I know. But it works...
	resp1, e := http.Get(url)
	if e != nil {
		fmt.Println("http.Get error:", e)
		log.Println(e)
	}
	//defer resp.Body.Close()
	filesReader = map[string]io.Reader{
		`files1`: resp1.Body,
	}
	return filesReader
}
