package cmd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
	"github.com/aldinokemal/go-whatsapp-web-multidevice/ui/rest/middleware"
	"github.com/stretchr/testify/require"
)

const authTestHost = "wa.ruangsanad.com"

func webTestApp(t *testing.T) (*fiber.App, *webSessionStore, map[string]string) {
	t.Helper()
	st, err := openWebSessionStore(filepath.Join(t.TempDir(), "ui-session.db"))
	require.NoError(t, err)
	t.Cleanup(func(){ require.NoError(t,st.Close()) })
	accounts:=map[string]string{"owner":"correct-password", "cs":"restricted-pass"}
	app:=fiber.New()
	app.Get("/health",func(c fiber.Ctx)error{return c.SendString("OK")})
	registerWebLogin(app,accounts,st,"")
	app.Use(middleware.WebsocketQueryAuth())
	app.Use(newWebOrBasicAuthMiddleware(accounts,st,""))
	app.Get("/",func(c fiber.Ctx)error{return c.SendString("Dashboard "+authUsername(c))})
	app.Get("/devices",func(c fiber.Ctx)error{return c.JSON(fiber.Map{"code":"SUCCESS"})})
	app.Get("/cs-ai",func(c fiber.Ctx)error{return c.SendString("CS:"+authUsername(c))})
	app.Get("/ws",func(c fiber.Ctx)error{return c.SendStatus(200)})
	app.Post("/send",func(c fiber.Ctx)error{return c.SendString(authUsername(c))})
	return app,st,accounts
}

func webCall(t *testing.T,app *fiber.App,method,path string,form url.Values,cookies ...*http.Cookie)(*http.Response,string) {
	t.Helper()
	var body io.Reader
	if form != nil {body=strings.NewReader(form.Encode())}
	req:=httptest.NewRequest(method,"https://"+authTestHost+path,body)
	req.Host=authTestHost
	if form != nil { req.Header.Set("Content-Type","application/x-www-form-urlencoded") }
	if method==http.MethodPost {req.Header.Set("Origin","https://"+authTestHost)}
	for _,c:= range cookies {req.AddCookie(c)}
	resp,err:=app.Test(req)
	require.NoError(t,err)
	defer resp.Body.Close()
	raw,err:=io.ReadAll(resp.Body)
	require.NoError(t,err)
	return resp,string(raw)
}

func findCookie(t *testing.T, resp *http.Response, name string) *http.Cookie {
	t.Helper()
	for _,cookie:=range resp.Cookies(){if cookie.Name==name{return cookie}}
	t.Fatalf("cookie %s not found",name)
	return nil
}

func loginCookie(t *testing.T,app *fiber.App) *http.Cookie{
	t.Helper()
	page,html:=webCall(t,app,"GET","/login",nil)
	require.Equal(t,200,page.StatusCode)
	require.Contains(t,html,"Masuk ke GoWA")
	require.NotContains(t,html,"correct-password")
	csrf:=findCookie(t,page,webCSRFCookie)
	require.True(t,csrf.Secure)
	form:=url.Values{"username":{"owner"},"password":{"correct-password"},"csrf":{csrf.Value}}
	resp,_:=webCall(t,app,"POST","/auth/login",form,csrf)
	require.Equal(t,302,resp.StatusCode)
	require.Equal(t,"/",resp.Header.Get("Location"))
	session:=findCookie(t,resp,webSessionCookie)
	require.True(t,session.HttpOnly)
	require.True(t,session.Secure)
	require.Equal(t,http.SameSiteLaxMode,session.SameSite)
	require.GreaterOrEqual(t,session.MaxAge, int(webSessionLifetime.Seconds())-5)
	return session
}

func TestWebLoginNoBasicPopupAndCookieAuth(t *testing.T){
	app,_,_:=webTestApp(t)
	resp,_:=webCall(t,app,"GET","/",nil)
	require.Equal(t,302,resp.StatusCode)
	require.Equal(t,"/login?error=expired",resp.Header.Get("Location"))
	require.Empty(t,resp.Header.Get("WWW-Authenticate"))
	resp,_=webCall(t,app,"GET","/devices",nil)
	require.Equal(t,401,resp.StatusCode)
	require.Empty(t,resp.Header.Get("WWW-Authenticate"))
	session:=loginCookie(t,app)
	resp,html:=webCall(t,app,"GET","/",nil,session)
	require.Equal(t,200,resp.StatusCode)
	require.Equal(t,"Dashboard owner",html)
	resp,body:=webCall(t,app,"GET","/cs-ai",nil,session)
	require.Equal(t,200,resp.StatusCode)
	require.Equal(t,"CS:owner",body)
	resp,_=webCall(t,app,"GET","/devices",nil,session)
	require.Equal(t,200,resp.StatusCode)
}

func TestWebSessionExpiresRotatesPasswordAndSurvivesRestart(t *testing.T) {
	path:=filepath.Join(t.TempDir(),"session.db")
	store,err:=openWebSessionStore(path)
	require.NoError(t,err)
	now:=time.Now()
	token,err:=store.Create(context.Background(),"owner","password-old",now)
	require.NoError(t,err)
	require.Len(t,token,43)
	require.Equal(t,14*24*time.Hour,webSessionLifetime)
	accounts:=map[string]string{"owner":"password-old"}
	_,valid:=store.Identity(context.Background(),token,accounts,now.Add(13*24*time.Hour))
	require.True(t,valid)
	_,valid=store.Identity(context.Background(),token,accounts,now.Add(14*24*time.Hour))
	require.False(t,valid)
	accounts["owner"]="password-new"
	_,valid=store.Identity(context.Background(),token,accounts,now)
	require.False(t,valid)
	accounts["owner"]="password-old"
	require.NoError(t,store.Close())
	store,err=openWebSessionStore(path)
	require.NoError(t,err)
	defer store.Close()
	_,valid=store.Identity(context.Background(),token,accounts,now)
	require.True(t,valid,"session must survive process restart")
	require.NoError(t,store.Revoke(context.Background(),token))
	_,valid=store.Identity(context.Background(),token,accounts,now)
	require.False(t,valid)
}

func TestWebAuthBasicStillWorksAndSessionCSRF(t *testing.T) {
	app,_,_:=webTestApp(t)
	req:=httptest.NewRequest("POST","https://"+authTestHost+"/send",nil)
	req.SetBasicAuth("owner","correct-password")
	res,err:=app.Test(req)
	require.NoError(t,err)
	require.Equal(t,200,res.StatusCode)
	res.Body.Close()

	bad:=httptest.NewRequest("GET","https://"+authTestHost+"/devices",nil)
	bad.SetBasicAuth("owner","wrong")
	res,err=app.Test(bad)
	require.NoError(t,err)
	require.Equal(t,401,res.StatusCode)
	require.Empty(t,res.Header.Get("WWW-Authenticate"))
	res.Body.Close()

	session:=loginCookie(t,app)
	post:=httptest.NewRequest("POST","https://"+authTestHost+"/send",nil)
	post.AddCookie(session)
	// No same-origin browser signal -> rejected despite valid session cookie.
	res,err=app.Test(post)
	require.NoError(t,err)
	require.Equal(t,403,res.StatusCode)
	res.Body.Close()
	post.Header.Set("Origin","https://evil.example.com")
	res,err=app.Test(post)
	require.NoError(t,err)
	require.Equal(t,403,res.StatusCode)
	res.Body.Close()
	post.Header.Set("Origin","https://"+authTestHost)
	res,err=app.Test(post)
	require.NoError(t,err)
	require.Equal(t,200,res.StatusCode)
	res.Body.Close()
}

func TestWebLoginRejectsCSRFAndRateLimits(t *testing.T) {
	app,_,_:=webTestApp(t)
	resp,_:=webCall(t,app,"POST","/auth/login",url.Values{"username":{"owner"},"password":{"correct-password"}})
	require.Equal(t,403,resp.StatusCode)
	for i:=0;i<webLoginLimit;i++{
		page,_:=webCall(t,app,"GET","/login",nil)
		csrf:=findCookie(t,page,webCSRFCookie)
		invalid,_:=webCall(t,app,"POST","/auth/login",url.Values{
			"username":{"owner"},"password":{"wrong"},"csrf":{csrf.Value},
		},csrf)
		require.Equal(t,302,invalid.StatusCode)
	}
	page,_:=webCall(t,app,"GET","/login",nil)
	csrf:=findCookie(t,page,webCSRFCookie)
	limited,_:=webCall(t,app,"POST","/auth/login",url.Values{
		"username":{"owner"},"password":{"correct-password"},"csrf":{csrf.Value},
	},csrf)
	require.Equal(t,"/login?error=limited",limited.Header.Get("Location"))
}

func TestWebLogoutRevokesSession(t *testing.T) {
	app,_,_:=webTestApp(t)
	session:=loginCookie(t,app)
	logoutPage,_:=webCall(t,app,"GET","/logout",nil,session)
	require.Equal(t,200,logoutPage.StatusCode)
	csrf:=findCookie(t,logoutPage,webCSRFCookie)
	done,_:=webCall(t,app,"POST","/auth/logout",url.Values{"csrf":{csrf.Value}},session,csrf)
	require.Equal(t,302,done.StatusCode)
	require.Equal(t,"/login",done.Header.Get("Location"))
	resp,_:=webCall(t,app,"GET","/",nil,session)
	require.Equal(t,302,resp.StatusCode)
}

func TestWebSessionWebSocketOrigin(t *testing.T){
	app,_,_:=webTestApp(t)
	session:=loginCookie(t,app)
	upgrade:=func(origin string)int{
		req:=httptest.NewRequest("GET","https://"+authTestHost+"/ws",nil)
		req.AddCookie(session)
		req.Header.Set("Upgrade","websocket")
		req.Header.Set("Connection","Upgrade")
		req.Header.Set("Origin",origin)
		resp,err:=app.Test(req)
		require.NoError(t,err)
		defer resp.Body.Close()
		return resp.StatusCode
	}
	require.Equal(t,403,upgrade("https://foreign.example"))
	require.Equal(t,200,upgrade("https://"+authTestHost))
}

func TestSessionAuthBasePath(t *testing.T){
	st,err:=openWebSessionStore(filepath.Join(t.TempDir(),"sessions.db"))
	require.NoError(t,err)
	defer st.Close()
	app:=fiber.New()
	accounts:=map[string]string{"owner":"pwd"}
	registerWebLogin(app,accounts,st,"/gowa")
	app.Use(newWebOrBasicAuthMiddleware(accounts,st,"/gowa"))
	app.Get("/gowa/",func(c fiber.Ctx)error{return c.SendStatus(200)})
	req:=httptest.NewRequest("GET","https://wa.example.com/gowa/",nil)
	res,err:=app.Test(req)
	require.NoError(t,err)
	defer res.Body.Close()
	require.Equal(t,302,res.StatusCode)
	require.Equal(t,"/gowa/login?error=expired",res.Header.Get("Location"))
}
