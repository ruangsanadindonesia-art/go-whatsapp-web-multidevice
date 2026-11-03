package cmd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	"github.com/gofiber/fiber/v3"
)

func TestPrivateAPIAdminUIAuthorizationAndOneTimeSecret(t *testing.T) {
	prevCredentials := config.AppBasicAuthCredential
	prevAdmins := config.AppIntegrationsAdminUsersRaw
	prevScopes := config.AppBasicAuthDevices
	t.Cleanup(func(){
		config.AppBasicAuthCredential=prevCredentials
		config.AppIntegrationsAdminUsersRaw=prevAdmins
		config.AppBasicAuthDevices=prevScopes
	})
	config.AppBasicAuthCredential=[]string{"admin:example","other:example"}
	config.AppIntegrationsAdminUsersRaw="admin"
	config.AppBasicAuthDevices=map[string][]string{}
	store,err:=openPrivateAPIStore(filepath.Join(t.TempDir(),"private.db"))
	if err!=nil{t.Fatal(err)}
	defer store.Close()
	user := "other"
	app:=fiber.New()
	app.Use(func(c fiber.Ctx) error {
		c.Locals(webAuthUserKey,user)
		return c.Next()
	})
	registerIntegrationsPage(app,store)
	registerPrivateAPIKeyAdmin(app,store,privateAPIBackend{
		Resolve:func(ctx context.Context,device string)(context.Context,string,error){
			if device=="device-1" {return ctx,device,nil}
			return ctx,"",context.Canceled
		},
	})
	request:=func(method,path,origin string,values url.Values)(int,string){
		t.Helper()
		var body io.Reader
		if values!=nil {body=strings.NewReader(values.Encode())}
		req:=httptest.NewRequest(method,path,body)
		req.Host="wa.example.com"
		if origin!="" {req.Header.Set("Origin",origin)}
		if values!=nil {req.Header.Set("Content-Type","application/x-www-form-urlencoded")}
		resp,err:=app.Test(req)
		if err!=nil{t.Fatal(err)}
		data,err:=io.ReadAll(resp.Body)
		_ =resp.Body.Close()
		if err!=nil{t.Fatal(err)}
		return resp.StatusCode,string(data)
	}
	form:=url.Values{"name":{"n8n Test"},"device_id":{"device-1"}}
	if status,_:=request("POST","/integrations/keys","https://wa.example.com",form);status!=403 {
		t.Fatalf("non-admin key creation allowed: %d",status)
	}
	user="admin"
	if status,_:=request("POST","/integrations/keys","https://other.example.com",form);status!=403 {
		t.Fatalf("cross-origin admin action allowed: %d",status)
	}
	status,created:=request("POST","/integrations/keys","https://wa.example.com",form)
	if status!=200 {t.Fatalf("admin key creation failed: %d %s",status,created)}
	matcher:=regexp.MustCompile("gwa_[a-f0-9]{12}_[a-zA-Z0-9_-]{43}")
	token:=matcher.FindString(created)
	if token=="" {t.Fatal("one-time key missing from creation response")}
	if !strings.Contains(created,"no") && !strings.Contains(created,"Simpan sekarang") {t.Fatal("missing one-time notice")}
	if _,allowed,err:=store.Authenticate(context.Background(),token);err!=nil || !allowed {t.Fatalf("created key invalid: %v",err)}
	if _,listBody:=request("GET","/integrations?tab=api","",nil);strings.Contains(listBody,token) {
		t.Fatal("plaintext key shown in management list")
	}
	keys,err:=store.List(context.Background())
	if err!=nil || len(keys)!=1{t.Fatalf("key list %v %v",keys,err)}
	id:=keys[0].ID
	if status,_:=request("POST","/integrations/keys/"+id+"/revoke","https://other.example.com",nil);status!=403 {
		t.Fatalf("cross-origin revoke allowed: %d",status)
	}
	if status,_:=request("POST","/integrations/keys/"+id+"/revoke","https://wa.example.com",nil);status!=http.StatusFound && status!=http.StatusSeeOther {
		t.Fatalf("admin revoke failed: %d",status)
	}
	if _,allowed,err:=store.Authenticate(context.Background(),token);err!=nil || allowed{t.Fatal("revoked token still active")}
	config.AppBasicAuthDevices=map[string][]string{"admin":{"device-1"}}
	if status,_:=request("POST","/integrations/keys","https://wa.example.com",form);status!=403{
		t.Fatalf("device-restricted admin created key: %d",status)
	}
}

func TestPrivateAPIKeyWithUnderscoreInSecret(t *testing.T){
	store,err:=openPrivateAPIStore(filepath.Join(t.TempDir(),"api-keys.db"))
	if err!=nil {t.Fatal(err)}
	defer store.Close()
	// The 32-byte base64url secret often contains an underscore, which must
	// not be mistaken for a separator between token components.
	found:=false
	for i:=0;i<40;i++ {
		_,token,err:=store.Create(context.Background(),"integration","device-1",false)
		if err!=nil{t.Fatal(err)}
		if strings.Count(token,"_")>=3 {
			found=true
			_,valid,err:=store.Authenticate(context.Background(),token)
			if err!=nil || !valid{t.Fatalf("valid key with underscore rejected: %v",err)}
			break
		}
	}
	if !found {t.Fatal("test could not generate underscore-bearing secret")}
}
