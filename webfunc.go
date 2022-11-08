package oom

// webfunc.go provides a single public function
// MustFetch(url string) data[] byte
// The package level client need not be accessed outside this file

import (
	"bufio"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	//"golang.org/x/net/publicsuffix"
	"errors"
	"fmt"
	"strings"
	"sync"
)

// single http client (will be set to valid logged in client on first call)
var client *http.Client
var mutex = &sync.Mutex{}

// MustFetch returns a byteslice for the given page or dies.  It invokes
// login() the first time it is called to set package variable client
// where login() reads email and pin from creds.conf by default or from stdin
func MustFetch(urlString string) (data []byte) {
	mutex.Lock()
	if client == nil {
		if err := login(); err != nil {
			log.Fatal(err)
		}
	}
	mutex.Unlock()
	return fetchPage(urlString)
}

func fetchPage(urlString string) (data []byte) {
	log.Println("fetching page ", urlString)
	u, err := url.Parse(urlString)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := client.Get(u.String())
	if err != nil {
		log.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Fatal("Server returned non-200 status: %v\n", resp.Status)
	}
	data, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	resp.Body.Close()
	return
}

// login sets package level *http.Client client to a valid logged in client
// to www.colchestergolfclub.com or returns an error.
// Credentials are read from file creds.conf or (if missing) from Stdin
// The returned page is checked for string "<title>Login Required" which if
// found indicates a failed login
func login() error {
	//log.Println("logging in...")
	options := cookiejar.Options{
	//PublicSuffixList: publicsuffix.List,
	}
	jar, err := cookiejar.New(&options)
	if err != nil {
		log.Println(err)
		return err
	}

	client = &http.Client{Jar: jar}
	u, err := url.Parse("https://www.colchestergolfclub.com/login.php")
	if err != nil {
		log.Println(err)
		return err
	}

	// first call to Get sets the session id - but not logged in yet
	resp, err := client.Get(u.String())
	if err != nil {
		log.Println(err)
		return err
	}
	resp.Body.Close()

	var email, pin string
	// try to read credentials from file
	f, err := os.Open("creds.conf")
	if err == nil {
		defer f.Close()
		scanner := bufio.NewScanner(f)
		scanner.Scan()
		email = scanner.Text()
		scanner.Scan()
		pin = scanner.Text()
	} else {
		email, pin = readCredsStdin()
	}
	fmt.Printf("Logging in using <%s>, <%s>\n", email, pin)

	// post the login data (discard returned page)
	resp, err = client.PostForm(u.String(),
		url.Values{"task": {"login"}, "topmenu": {"1"},
			"memberid": {email}, "pin": {pin},
			"cachemid": {"1"}, "Submit": {"Login"}})

	if err != nil {
		log.Println(err)
		return err
	}

	// check if the login OK
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	page := string(data)
	if strings.Index(page, "<title>Login Required") != -1 {
		//fmt.Println(page)
		return errors.New("Login failed - check credentials?")
	}

	resp.Body.Close()
	return nil
}

func readCredsStdin() (email string, pin string) {
	//fmt.Println("readCredsStdin")
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Print("Enter email: ")
	scanner.Scan()
	email = scanner.Text()
	fmt.Print("Enter PIN: ")
	scanner.Scan()
	pin = scanner.Text()
	//fmt.Printf("readCredsStdin email  <%s>, pin <%s>\n", email, pin)
	//fmt.Println(pin)
	return
}
