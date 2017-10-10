// +build go1.8

package main

import (
	"log"
	"net/http"
	"net/url"
	"crypto/tls"
	"fmt"
	"time"
	"context"
	"os"
	"os/signal"
	"io/ioutil"

	"github.com/gin-gonic/gin"
)

var configAddress     = "0.0.0.0:8080" // 0.0.0.0:8080
var configFreeIpaHost = "ldap.muratyaman.co.uk"

func main() {
	log.Println("main() start")

	log.Println("main() define router")
	router := gin.Default()


	// api group
	api := router.Group("/api")
	{
		api.GET("/users/ping", pingHandler)
		api.POST("/users/login", userLogin)
		//api.GET("/users/me",    userMe)
		//api.POST("/users",      userCreate)
		//api.GET("/users/:uuid", userRetrieve)
	}

	// listen and serve on address
	srv := &http.Server{
		Addr:         configAddress,
		Handler:      router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
	}

	// code for graceful shutdown

	// run async lambda
	go func() {
		// response to requests
		if err := srv.ListenAndServe(); err != nil {
			log.Printf("listen: %s\n", err)
		}
	}()

	// Wait for interrupt signal to gracefully shutdown the server with
	// a timeout of 5 seconds.

	quit := make(chan os.Signal)
	signal.Notify(quit, os.Interrupt)
	<-quit
	log.Println("Shutdown Server ...")

	ctx, cancel := context.WithTimeout(context.Background(), 5 * time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server Shutdown:", err)
	}


	log.Println("main() Server exiting")
}

func pingHandler(ctx *gin.Context) {
	now := time.Now()
	nowStr := now.Format("2006-01-02 03:04:05pm-07")
	output := gin.H{ "message": fmt.Sprintf("pong at %s", nowStr) }
	ctx.JSON(http.StatusOK, output)
}

// Binding from JSON
type UserLoginInput struct {
	Username string `json:"username" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// for info
type UserLoginOutput struct {
	error   string
	token   string
}

func userLogin(ctx *gin.Context) {
	log.Println("userLogin() start")
	var json UserLoginInput

	status := http.StatusBadRequest
	output := gin.H{"error": "bad request"}

	if ctx.BindJSON(&json) == nil {
		ipaInput  := freeIpaLoginInput{User: json.Username, Password: json.Password}
		ipaOutput := freeIpaLogin(ipaInput)

		if ipaOutput.Token != "" {
			status = http.StatusOK
			output = gin.H{"token": ipaOutput.Token }

			//TODO after successful login, (async) create/update user record in the local db

		} else {
			status = http.StatusUnauthorized
			output = gin.H{"error": "unauthorized"}
		}
	}

	ctx.JSON(status, output)
	log.Println("userLogin() end")
}


// FreeIPA HTTP Client
func freeIpaHttpClient() *http.Client {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // ignore expired SSL certificates
		},
	}
	client := &http.Client{Transport: tr}
	return client
}

// Input for FreeIPA login action - used internally
type freeIpaLoginInput struct {
	User string
	Password string
}

// Output for FreeIPA login action - used internally
type freeIpaLoginOutput struct {
	Error string
	Token string
}

func freeIpaLogin(input freeIpaLoginInput) freeIpaLoginOutput {
	log.Println("freeIpaLogin() start")
	output := freeIpaLoginOutput{}

	client := freeIpaHttpClient()

	targetUrl := fmt.Sprintf("https://%s/ipa/session/login_password", configFreeIpaHost)

	data := url.Values{
		"user": {input.User},
		"password": {input.Password},
	}
	response, err := client.PostForm(targetUrl, data)

	if err != nil {
		log.Fatal(err)
		output.Error = err.Error()
		log.Println("freeIpaLogin() end with error")
		return output
	}

	defer response.Body.Close()

	bodyHtml, err := ioutil.ReadAll(response.Body)
	log.Println(string(bodyHtml))

	// e.g. 'Set-Cookie:ipa_session=54722a0c70374437893a9964b110f0f5; Domain=ldap.muratyaman.co.uk; Path=/ipa; Expires=Tue, 10 Oct 2017 20:55:30 GMT; Secure; HttpOnly'
	cookies := response.Cookies()

	output.Token = getCookieByName(cookies, "ipa_session")
	log.Println("freeIpaLogin() end")
	return output
}

func getCookieByName(cookies []*http.Cookie, name string) string {
	log.Println(fmt.Sprintf("getCookieByName(%s) start", name))
	result := ""
	for _, cookie := range cookies {
		if cookie.Name == name {
			result = cookie.Value
			break // found it, exit loop
		}
	}
	log.Println(fmt.Sprintf("getCookieByName(%s) end ==> %s", name, result))
	return result
}