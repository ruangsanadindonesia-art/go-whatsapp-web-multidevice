package cmd

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aldinokemal/go-whatsapp-web-multidevice/config"
	domainSend "github.com/aldinokemal/go-whatsapp-web-multidevice/domains/send"
	"github.com/gofiber/fiber/v3"
)

func TestPrivateAPIKeyLifecycleAndPersistence(t *testing.T) {
	path := filepath.Join(t.TempDir(),"api-keys.db")
	store, err := openPrivateAPIStore(path)
	if err != nil { t.Fatal(err) }

	key, token, err := store.Create(context.Background(),"n8n test","device-alpha",false)
	if err != nil { t.Fatal(err) }
	if !strings.HasPrefix(token,"gwa_") { t.Fatalf("missing token prefix") }
	if key.CanGroup { t.Fatal("group should default deny") }
	if strings.Contains(key.Prefix,token) { t.Fatal("stored prefix equals secret") }
	var foundHash string
	if err := store.db.QueryRow("SELECT token_hash FROM private_api_keys WHERE id=?",key.ID).Scan(&foundHash); err != nil { t.Fatal(err) }
	if foundHash == token || len(foundHash) != 64 { t.Fatal("plaintext key leaked to database") }
	list, err := store.List(context.Background())
	if err != nil || len(list) != 1 || list[0].Name != "n8n test" { t.Fatalf("list failed %v %v",list,err) }
	if strings.Contains(list[0].Prefix,token) { t.Fatal("raw key exposed by List") }

	if err := store.Close(); err != nil { t.Fatal(err) }
	store, err = openPrivateAPIStore(path)
	if err != nil { t.Fatal(err) }
	defer store.Close()
	_, ok, err := store.Authenticate(context.Background(),token)
	if err != nil || !ok { t.Fatalf("persisted key not valid: %v",err) }
	_, ok, err = store.Authenticate(context.Background(),token+"forged")
	if err != nil || ok { t.Fatal("forged token accepted") }
	_, ok, err = store.Authenticate(context.Background(),"gwa_wrong")
	if err != nil || ok { t.Fatal("malformed token accepted") }
	if success,err := store.Revoke(context.Background(),key.ID); err != nil || !success { t.Fatalf("revoke: %v",err) }
	_, ok, err = store.Authenticate(context.Background(),token)
	if err != nil || ok { t.Fatal("revoked key accepted") }
	if success,err := store.Revoke(context.Background(),key.ID); err != nil || success { t.Fatal("double revoke succeeded") }

	expired, expiredRaw, err := store.Create(context.Background(),"expired","device-alpha",true)
	if err != nil { t.Fatal(err) }
	if _,err := store.db.Exec("UPDATE private_api_keys SET expires_at=1 WHERE id=?",expired.ID); err != nil { t.Fatal(err) }
	_, ok, err = store.Authenticate(context.Background(),expiredRaw)
	if err != nil || ok { t.Fatal("expired key accepted") }
}

func TestPrivateAPIAuthDeviceGroupIdempotencyAndIsolation(t *testing.T) {
	previous := config.AppBasicAuthCredential
	config.AppBasicAuthCredential = []string{"admin:legacy-password"}
	t.Cleanup(func(){config.AppBasicAuthCredential=previous})
	store, err := openPrivateAPIStore(filepath.Join(t.TempDir(),"api-keys.db"))
	if err != nil { t.Fatal(err) }
	defer store.Close()
	personal, personalToken, err := store.Create(context.Background(),"personal","dev-one",false)
	if err != nil { t.Fatal(err) }
	_, groupToken, err := store.Create(context.Background(),"group","dev-one",true)
	if err != nil { t.Fatal(err) }
	sends := 0
	backend := privateAPIBackend{
		Resolve: func(ctx context.Context, requested string)(context.Context,string,error){
			if requested=="dev-one" || requested=="dev-two" {return ctx,requested,nil}
			return ctx,"",context.DeadlineExceeded
		},
		Send: func(ctx context.Context, req domainSend.MessageRequest)(domainSend.GenericResponse,error){
			sends++
			return domainSend.GenericResponse{MessageID:"test-message",Status:"success"},nil
		},
		Status: func(ctx context.Context,id string)(bool,bool,error){return true,true,nil},
	}
	app := fiber.New()
	registerPrivateIntegrationAPI(app,"",store,backend)
	app.Use(newBasicAuthMiddleware(map[string]string{"admin":"legacy-password"}))
	app.Get("/legacy",func(c fiber.Ctx)error{return c.SendStatus(200)})

	call := func(method,url,token,device,idem,payload string)(int,map[string]any){
		t.Helper()
		req := httptest.NewRequest(method,url,strings.NewReader(payload))
		if token!="" {req.Header.Set("Authorization","Bearer "+token)}
		if device!="" {req.Header.Set("X-Device-Id",device)}
		if idem!="" {req.Header.Set("Idempotency-Key",idem)}
		if payload!="" {req.Header.Set("Content-Type","application/json")}
		response, err := app.Test(req)
		if err != nil {t.Fatal(err)}
		data,err := io.ReadAll(response.Body)
		_ = response.Body.Close()
		if err != nil {t.Fatal(err)}
		var output map[string]any
		if len(data)>0 && strings.HasPrefix(string(data),"{") {
			if err := json.Unmarshal(data,&output); err != nil {t.Fatal(err)}
		}
		return response.StatusCode,output
	}
	private := "gwa_invalid"
	if code,_:=call("GET","/api/v1/devices/dev-one/status",private,"","","");code!=401{t.Fatalf("invalid token status %d",code)}
	if code,_:=call("GET","/api/v1/devices/dev-two/status",personalToken,"","","");code!=403{t.Fatalf("cross-device status %d",code)}
	if code,_:=call("GET","/api/v1/devices/dev-one/status",personalToken,"","","");code!=200{t.Fatalf("scoped device status %d",code)}
	groupBody := `{"phone":"12345678901@g.us","message":"Hello group"}`
	if code,_:=call("POST","/api/v1/messages",personalToken,"dev-one","req-00000001",groupBody);code!=403{
		t.Fatalf("ungranted group send %d",code)
	}
	if sends!=0 {t.Fatal("group send reached backend without grant")}
	body := `{"phone":"628123456789","message":"Hello personal"}`
	if code,_:=call("POST","/api/v1/messages",personalToken,"dev-two","req-00000002",body);code!=403{
		t.Fatalf("cross-device send %d",code)
	}
	if code,_:=call("POST","/api/v1/messages",personalToken,"dev-one","",body);code!=400{
		t.Fatalf("missing idempotency %d",code)
	}
	code,result:=call("POST","/api/v1/messages",personalToken,"dev-one","req-00000003",body)
	if code!=200 || result["state"]!="SENT" || sends!=1 {t.Fatalf("valid send: %d %v sends=%d",code,result,sends)}
	requestID, ok := result["request_id"].(string)
	if !ok || requestID=="" { t.Fatalf("missing request id: %v",result) }
	if code,replayed:=call("POST","/api/v1/messages",personalToken,"dev-one","req-00000003",body);code!=200 || replayed["replayed"]!=true || sends!=1 {
		t.Fatalf("duplicate sent again: %d %v sends=%d",code,replayed,sends)
	}
	diff := `{"phone":"628123456789","message":"Different"}`
	if code,_:=call("POST","/api/v1/messages",personalToken,"dev-one","req-00000003",diff);code!=409 || sends!=1 {
		t.Fatalf("idempotency conflict: %d sends=%d",code,sends)
	}
	if code,_:=call("GET","/api/v1/messages/"+requestID,personalToken,"","","");code!=200{t.Fatalf("own status %d",code)}
	if code,_:=call("GET","/api/v1/messages/"+requestID,groupToken,"","","");code!=404{t.Fatalf("cross-client status %d",code)}
	if code,_:=call("POST","/api/v1/messages",groupToken,"dev-one","req-00000004",groupBody);code!=200 || sends!=2 {
		t.Fatalf("allowed group not sent: %d sends=%d",code,sends)
	}
	if code,_:=call("GET","/legacy",personalToken,"","","");code!=401 {
		t.Fatalf("API key authenticated to legacy REST: %d",code)
	}
	if _,err:=store.Revoke(context.Background(),personal.ID);err!=nil{t.Fatal(err)}
	if code,_:=call("GET","/api/v1/devices/dev-one/status",personalToken,"","","");code!=401 {
		t.Fatalf("revoked API key still works: %d",code)
	}
}

func TestPrivateAPIRecipientAndIdempotencyValidation(t *testing.T) {
	for _, v:=range []string{"status@broadcast","123@newsletter","wrong@g.us","62812@lid","javascript:alert(1)",""} {
		if _,_,ok:=privateAPINormalizeRecipient(v);ok{t.Fatalf("unexpected allowed recipient %s",v)}
	}
	if recipient,group,ok:=privateAPINormalizeRecipient("120363012345678901@g.us");!ok || !group || recipient=="" {
		t.Fatal("valid group rejected")
	}
	for _, value:=range []string{"abc","x",strings.Repeat("a",129)} {
		if privateAPIIdempotencyPattern.MatchString(value) {t.Fatalf("invalid key accepted: %q",value)}
	}
}
