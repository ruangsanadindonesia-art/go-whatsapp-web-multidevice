package cmd

import (
 "context"
 "database/sql"
 "encoding/json"
 "io"
 "net/http/httptest"
 "path/filepath"
 "strings"
 "testing"

 "github.com/aldinokemal/go-whatsapp-web-multidevice/config"
 uimcp "github.com/aldinokemal/go-whatsapp-web-multidevice/ui/mcp"
 "github.com/gofiber/fiber/v3"
 "github.com/aldinokemal/go-whatsapp-web-multidevice/pkg/sqlite"
)

func TestPrivateToolGrantsPersistence(t *testing.T) {
 path:=filepath.Join(t.TempDir(),"keys.db")
 store,err:=openPrivateAPIStore(path)
 if err!=nil {t.Fatal(err)}
 _,oldToken,err:=store.Create(context.Background(),"old-client","device-1",false)
 if err!=nil {t.Fatal(err)}
 _,token,err:=store.Create(context.Background(),"trusted","device-1",true,privateAPIToolGrants{REST:true,MCP:true})
 if err!=nil {t.Fatal(err)}
 if err:=store.Close();err!=nil {t.Fatal(err)}
 reopened,err:=openPrivateAPIStore(path)
 if err!=nil {t.Fatal(err)}
 defer reopened.Close()
 old,ok,err:=reopened.Authenticate(context.Background(),oldToken)
 if err!=nil || !ok || old.CanRESTTools || old.CanMCPTools {
  t.Fatalf("old key gained privileges: %v %v %+v",ok,err,old)
 }
 grant,ok,err:=reopened.Authenticate(context.Background(),token)
 if err!=nil || !ok || !grant.CanRESTTools || !grant.CanMCPTools || !grant.CanGroup {
  t.Fatalf("new grants lost: %v %v %+v",ok,err,grant)
 }
 listed,err:=reopened.List(context.Background())
 if err!=nil || len(listed)!=2 {t.Fatalf("list %v %v",listed,err)}
}

func TestPrivateLegacyRESTToolScope(t *testing.T) {
 old:=config.AppBasicAuthCredential
 config.AppBasicAuthCredential=[]string{"admin:test"}
 t.Cleanup(func(){config.AppBasicAuthCredential=old})
 store,err:=openPrivateAPIStore(filepath.Join(t.TempDir(),"keys.db"))
 if err!=nil {t.Fatal(err)}
 defer store.Close()
 _,token,err:=store.Create(context.Background(),"rest","dev-one",false,privateAPIToolGrants{REST:true})
 if err!=nil {t.Fatal(err)}
 _,noGrant,err:=store.Create(context.Background(),"no-rest","dev-one",false)
 if err!=nil {t.Fatal(err)}
 _,groupGrant,err:=store.Create(context.Background(),"group-rest","dev-one",true,privateAPIToolGrants{REST:true})
 if err!=nil {t.Fatal(err)}
 backend:=privateAPIBackend{Resolve:func(ctx context.Context,id string)(context.Context,string,error){
  switch id {
  case "dev-one","alias-one":return ctx,"dev-one",nil
  case "dev-two":return ctx,"dev-two",nil
  }
  return ctx,"",context.Canceled
 }}
 app:=fiber.New()
 app.Use(newPrivateLegacyToolAuth(store,backend,""))
 app.Use(func(c fiber.Ctx)error {
  if c.Locals("private_legacy_authorized")==true {return c.Next()}
  return c.SendStatus(401)
 })
 for _,path:=range []string{"/send/message","/group","/integrations","/cs-ai","/devices/dev-one/status","/devices/dev-two/status","/devices/dev-one/webhook","/app/devices","/app/status","/chat/120363012345678901@g.us/archive","/chat/628123456789@s.whatsapp.net/archive"} {
  app.Use(path,func(c fiber.Ctx)error{return c.SendStatus(200)})
 }
 check:=func(method,path,key,device,body string)int{
  t.Helper()
  request:=httptest.NewRequest(method,path,strings.NewReader(body))
  request.Header.Set("Authorization","Bearer "+key)
  if device!="" {request.Header.Set("X-Device-Id",device)}
  if body!="" {request.Header.Set("Content-Type","application/json")}
  resp,err:=app.Test(request)
  if err!=nil{t.Fatal(err)}
  _=resp.Body.Close()
  return resp.StatusCode
 }
 personal:="{\"phone\":\"628123456789\",\"message\":\"hello\"}"
 group:="{\"phone\":\"120363012345678901@g.us\",\"message\":\"hello\"}"
 for _,tc:=range []struct{method,path,key,device,body string;want int}{
  {"POST","/send/message",token,"alias-one",personal,200},
  {"POST","/send/message",token,"dev-two",personal,403},
  {"POST","/send/message",token,"",group,403},
  {"POST","/send/message",groupGrant,"",group,200},
  {"POST","/send/message",noGrant,"",personal,403},
  {"GET","/group",token,"","",403},
  {"GET","/group",groupGrant,"","",200},
  {"GET","/integrations",token,"","",403},
  {"GET","/cs-ai",token,"","",403},
  {"GET","/devices/dev-one/webhook",token,"","",403},
  {"GET","/devices/dev-two/status",token,"","",403},
  {"GET","/devices/dev-one/status",token,"","",200},
  {"GET","/app/devices",token,"","",403},
  {"GET","/app/status",token,"","",200},
  {"POST","/send/schedules/sched-one/resume",token,"","",403},
  {"POST","/chat/120363012345678901@g.us/archive",token,"","{\"archived\":true,\"phone\":\"628123456789\"}",403},
  {"POST","/chat/628123456789@s.whatsapp.net/archive",token,"","{\"archived\":true}",200},
 } {
  if got:=check(tc.method,tc.path,tc.key,tc.device,tc.body);got!=tc.want {t.Errorf("%s %s expected %d, got %d",tc.method,tc.path,tc.want,got)}
 }
}

func TestPrivateMCPAllSixCategoriesAndApproval(t *testing.T) {
 backend:=privateAPIBackend{Resolve:func(ctx context.Context,id string)(context.Context,string,error){
  if id=="allowed" || id=="alias" {return ctx,"allowed",nil}
  return ctx,"other",nil
 }}
 key:=privateAPIKey{DeviceID:"allowed",CanMCPTools:true,CanGroup:true}
 for _,tc:=range []struct{name,args,want string}{
  {"whatsapp_send","{\"phone\":\"628123456789\",\"type\":\"text\"}",""},
  {"whatsapp_schedule","{\"action\":\"list\"}",""},
  {"whatsapp_message","{\"action\":\"mark_read\",\"phone\":\"628123456789\"}",""},
  {"whatsapp_chat","{\"action\":\"list_chats\"}",""},
  {"whatsapp_group","{\"action\":\"info\",\"group_id\":\"120363012345678901@g.us\"}",""},
  {"whatsapp_app","{\"action\":\"status\"}",""},
  {"whatsapp_send","{\"phone\":\"120363012345678901@g.us\",\"type\":\"text\"}","MCP_GROUP_APPROVAL_REQUIRED"},
  {"whatsapp_group","{\"action\":\"remove_participants\"}","MCP_GROUP_APPROVAL_REQUIRED"},
  {"whatsapp_group","{\"action\":\"invite_link\",\"reset\":true}","MCP_GROUP_APPROVAL_REQUIRED"},
  {"whatsapp_schedule","{\"action\":\"resume\"}","MCP_GROUP_APPROVAL_REQUIRED"},
  {"whatsapp_send","{\"phone\":\"628123456789\",\"device_id\":\"other\"}","DEVICE_NOT_ALLOWED"},
  {"whatsapp_send","{\"phone\":\"status@broadcast\"}","MCP_RECIPIENT_NOT_ALLOWED"},
  {"unknown_tool","{}","MCP_TOOL_NOT_ALLOWED"},
 } {
  raw:=json.RawMessage("{\"name\":\""+tc.name+"\",\"arguments\":"+tc.args+"}")
  if got:=privateMCPToolPolicy(raw,key,backend);got!=tc.want {
   t.Errorf("%s %s => %q expected %q",tc.name,tc.args,got,tc.want)
  }
 }
}

func TestPrivateMCPTransportPreAuth(t *testing.T) {
 old:=config.AppBasicAuthCredential
 oldMCP:=config.McpEnabled
 config.AppBasicAuthCredential=[]string{"admin:secret"}
 config.McpEnabled=true
 t.Cleanup(func(){config.AppBasicAuthCredential=old;config.McpEnabled=oldMCP})
 store,err:=openPrivateAPIStore(filepath.Join(t.TempDir(),"keys.db"))
 if err!=nil {t.Fatal(err)}
 defer store.Close()
 _,noGrant,err:=store.Create(context.Background(),"old","one",false)
 if err!=nil {t.Fatal(err)}
 _,canMCP,err:=store.Create(context.Background(),"mcp","one",false,privateAPIToolGrants{MCP:true})
 if err!=nil {t.Fatal(err)}
 app:=fiber.New()
 backend:=privateAPIBackend{Resolve:func(ctx context.Context,id string)(context.Context,string,error){return ctx,"one",nil}}
 registerPrivateKeyMCP(app,nil,store,backend,"",uimcp.Deps{})
 request:=func(key,body string)int {
  req:=httptest.NewRequest("POST","/api/v1/mcp",strings.NewReader(body))
  req.Header.Set("Authorization","Bearer "+key)
  req.Header.Set("Content-Type","application/json")
  resp,err:=app.Test(req)
  if err!=nil {t.Fatal(err)}
  _,_=io.Copy(io.Discard,resp.Body)
  _=resp.Body.Close()
  return resp.StatusCode
 }
 mutation:="{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/call\",\"params\":{\"name\":\"whatsapp_group\",\"arguments\":{\"action\":\"create\"}}}"
 if got:=request(noGrant,mutation);got!=403 {t.Errorf("non-MCP key accepted: %d",got)}
 if got:=request(canMCP,mutation);got!=403 {t.Errorf("group mutation allowed: %d",got)}
 if got:=request(canMCP,"[{\"jsonrpc\":\"2.0\",\"id\":1,\"method\":\"tools/list\"}]");got!=400 {t.Errorf("MCP JSON batch accepted: %d",got)}
}

func TestPrivateToolSchemaUpgradeOldSQLite(t *testing.T) {
 path:=filepath.Join(t.TempDir(),"legacy.db")
 db,err:=sql.Open(sqlite.DriverName,sqlite.FormatChatStorageURI("file:"+filepath.ToSlash(path),true,true))
 if err!=nil{t.Fatal(err)}
 // Exact pre-feature key schema lacks the two new tool grants.
 _,err=db.Exec("CREATE TABLE private_api_keys ("+
  "id TEXT PRIMARY KEY, name TEXT NOT NULL, prefix TEXT NOT NULL UNIQUE,"+
  "token_hash TEXT NOT NULL UNIQUE, device_id TEXT NOT NULL, can_send_group INTEGER NOT NULL DEFAULT 0,"+
  "created_at INTEGER NOT NULL, expires_at INTEGER NOT NULL, revoked_at INTEGER)")
 if err!=nil{t.Fatal(err)}
 if err:=db.Close();err!=nil{t.Fatal(err)}
 store,err:=openPrivateAPIStore(path)
 if err!=nil{t.Fatalf("upgrade failed: %v",err)}
 defer store.Close()
 _,token,err:=store.Create(context.Background(),"legacy-compatible","one",false)
 if err!=nil{t.Fatal(err)}
 key,valid,err:=store.Authenticate(context.Background(),token)
 if err!=nil || !valid || key.CanRESTTools || key.CanMCPTools {
  t.Fatalf("migration should retain default-deny: %v %v %+v",valid,err,key)
 }
}


func TestPrivateLegacyRESTSendRateLimit(t *testing.T) {
 previous:=config.AppBasicAuthCredential
 config.AppBasicAuthCredential=[]string{"admin:example"}
 t.Cleanup(func(){config.AppBasicAuthCredential=previous})
 store,err:=openPrivateAPIStore(filepath.Join(t.TempDir(),"rate.db"))
 if err!=nil {t.Fatal(err)}
 defer store.Close()
 _,token,err:=store.Create(context.Background(),"rate-test","dev-one",false,privateAPIToolGrants{REST:true})
 if err!=nil {t.Fatal(err)}
 backend:=privateAPIBackend{Resolve:func(ctx context.Context,id string)(context.Context,string,error){
  return ctx,"dev-one",nil
 }}
 app:=fiber.New()
 app.Use(newPrivateLegacyToolAuth(store,backend,""))
 app.Use(func(c fiber.Ctx)error{return c.SendStatus(200)})
 for i:=0;i<31;i++ {
  req:=httptest.NewRequest("POST","/send/message",strings.NewReader("{\"phone\":\"628123456789\",\"message\":\"hello\"}"))
  req.Header.Set("Authorization","Bearer "+token)
  req.Header.Set("Content-Type","application/json")
  resp,err:=app.Test(req)
  if err!=nil {t.Fatal(err)}
  _=resp.Body.Close()
  want:=200
  if i==30 {want=429}
  if resp.StatusCode!=want {t.Fatalf("request %d got %d, expected %d",i+1,resp.StatusCode,want)}
 }
}
