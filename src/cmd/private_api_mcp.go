package cmd

import (
 "bytes"
 "context"
 "encoding/json"
 "io"
 "strings"
 "time"

 "github.com/aldinokemal/go-whatsapp-web-multidevice/config"
 "github.com/aldinokemal/go-whatsapp-web-multidevice/infrastructure/whatsapp"
 uimcp "github.com/aldinokemal/go-whatsapp-web-multidevice/ui/mcp"
 "github.com/gofiber/fiber/v3"
)

// API-key MCP transport is isolated from the existing /mcp OAuth+Basic route.
// It exposes the same six MCP definitions but checks the key and tool/device
// policy *before* forwarding JSON-RPC to mcp-go.
func registerPrivateKeyMCP(app *fiber.App, dm *whatsapp.DeviceManager, store *privateAPIStore, backend privateAPIBackend, base string, deps uimcp.Deps) {
 if store == nil || !config.McpEnabled { return }
 prefix:=strings.TrimRight(base,"/")+"/api/v1"
 app.Use(prefix+"/mcp",func(c fiber.Ctx) error {
  if len(config.AppBasicAuthCredential)==0 { return privateAPIError(c,503,"INTEGRATION_DISABLED") }
  header:=strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
  if !strings.HasPrefix(header,"Bearer gwa_") { return privateAPIError(c,401,"API_KEY_REQUIRED") }
  key,valid,err:=store.Authenticate(c.Context(),strings.TrimPrefix(header,"Bearer "))
  if err!=nil {return privateAPIError(c,503,"KEY_STORE_UNAVAILABLE")}
  if !valid {return privateAPIError(c,401,"INVALID_API_KEY")}
  if !key.CanMCPTools {return privateAPIError(c,403,"MCP_TOOLS_NOT_GRANTED")}
  if backend.Resolve==nil {return privateAPIError(c,503,"DEVICE_MANAGER_UNAVAILABLE")}
  requested:=strings.TrimSpace(c.Get("X-Device-Id"))
  if requested=="" {requested=key.DeviceID}
  _,canonical,err:=backend.Resolve(c.Context(),requested)
  if err!=nil || canonical!=key.DeviceID {return privateAPIError(c,403,"DEVICE_NOT_ALLOWED")}
  if c.Query("device_id")!="" {return privateAPIError(c,403,"DEVICE_NOT_ALLOWED")}
  c.Request().Header.Set("X-Device-Id",key.DeviceID)
  if c.Method()==fiber.MethodPost {
   if len(c.Body())==0 || len(c.Body())>64*1024 { return privateAPIError(c,400,"INVALID_MCP_PAYLOAD") }
   if !strings.HasPrefix(strings.ToLower(c.Get("Content-Type")),"application/json") {
    return privateAPIError(c,415,"MCP_JSON_REQUIRED")
   }
   var rpc struct {
    JSONRPC string `json:"jsonrpc"`
    Method string `json:"method"`
    Params json.RawMessage `json:"params"`
   }
   decoder:=json.NewDecoder(bytes.NewReader(c.Body()))
   if err:=decoder.Decode(&rpc);err!=nil {return privateAPIError(c,400,"INVALID_MCP_PAYLOAD")}
   var trailing any
   if err:=decoder.Decode(&trailing);err!=io.EOF {return privateAPIError(c,400,"INVALID_MCP_PAYLOAD")}
   if rpc.JSONRPC!="2.0" {return privateAPIError(c,400,"INVALID_MCP_VERSION")}
   switch rpc.Method {
   case "initialize","notifications/initialized","ping","tools/list":
    // No tool execution.
   case "tools/call":
    if !store.AllowSend(key.ID,time.Now()) {return privateAPIError(c,429,"MCP_RATE_LIMITED")}
    if errCode:=privateMCPToolPolicy(rpc.Params,key,backend);errCode!="" {
     return privateAPIError(c,403,errCode)
    }
   default:
    return privateAPIError(c,403,"MCP_METHOD_NOT_ALLOWED")
   }
  } else if c.Method()!=fiber.MethodDelete {
   return privateAPIError(c,405,"MCP_METHOD_NOT_ALLOWED")
  }
  c.Set("Cache-Control","no-store")
  return c.Next()
 })
 mcpRouter:=app.Group(prefix)
 uimcp.Register(mcpRouter,dm,deps)
}

// Strict JSON-RPC tool policy. Never accept tool calls embedded in batches,
// nested device IDs, or silently fall back to a default unrestricted device.
func privateMCPToolPolicy(raw json.RawMessage, key privateAPIKey, backend privateAPIBackend) string {
 var params struct {
  Name string `json:"name"`
  Arguments map[string]json.RawMessage `json:"arguments"`
 }
 if len(raw)==0 || json.Unmarshal(raw,&params)!=nil { return "INVALID_MCP_TOOL_CALL" }
 switch params.Name {
 case "whatsapp_send","whatsapp_schedule","whatsapp_message","whatsapp_chat","whatsapp_group","whatsapp_app":
 default: return "MCP_TOOL_NOT_ALLOWED"
 }
 for _,field:=range []string{"device_id","phone","group_id","chat_jid","action","type"} {
  if v,ok:=params.Arguments[field];ok {
   var asString string
   if json.Unmarshal(v,&asString)!=nil {return "INVALID_MCP_ARGUMENTS"}
  }
 }
 deviceID:=privateMCPArg(params.Arguments,"device_id")
 if deviceID!="" {
  _,resolved,err:=backend.Resolve(context.Background(),deviceID)
  if err!=nil || resolved!=key.DeviceID {return "DEVICE_NOT_ALLOWED"}
 }
 phone:=privateMCPArg(params.Arguments,"phone")
 groupID:=privateMCPArg(params.Arguments,"group_id")
 chatJID:=privateMCPArg(params.Arguments,"chat_jid")
 if params.Name=="whatsapp_send" {
  if phone=="" { return "INVALID_MCP_RECIPIENT" }
  // API-key MCP messages to groups require a human approval workflow that
  // has not yet been implemented. The REST v1 group grant does not bypass it.
  if strings.HasSuffix(phone,"@g.us") {return "MCP_GROUP_APPROVAL_REQUIRED"}
  if !privateMCPRecipientPersonal(phone) {return "MCP_RECIPIENT_NOT_ALLOWED"}
 }
 if params.Name=="whatsapp_group" {
  action:=privateMCPArg(params.Arguments,"action")
  switch action {
  case "info","participants","join_requests":
  case "invite_link":
   if value,ok:=params.Arguments["reset"];ok && string(value)!="false" {return "MCP_GROUP_APPROVAL_REQUIRED"}
  default: return "MCP_GROUP_APPROVAL_REQUIRED"
  }
 }
 if params.Name=="whatsapp_schedule" {
  // A schedule ID can identify a group send even when the recipient is
  // absent from this tool call. Until an approval service can validate the
  // schedule target, only discovery is safe for API-key-driven MCP.
  action:=privateMCPArg(params.Arguments,"action")
  if action!="list" && action!="get" {return "MCP_GROUP_APPROVAL_REQUIRED"}
 }
 if params.Name=="whatsapp_message" && strings.HasSuffix(phone,"@g.us") {
  return "MCP_GROUP_APPROVAL_REQUIRED"
 }
 if params.Name=="whatsapp_chat" && strings.HasSuffix(chatJID,"@g.us") && privateMCPArg(params.Arguments,"action")=="archive" {
  return "MCP_GROUP_APPROVAL_REQUIRED"
 }
 if strings.HasSuffix(groupID,"@g.us") && params.Name!="whatsapp_group" {
  return "MCP_GROUP_APPROVAL_REQUIRED"
 }
 return ""
}

func privateMCPArg(args map[string]json.RawMessage,key string) string {
 var value string
 _=json.Unmarshal(args[key],&value)
 return strings.TrimSpace(value)
}
func privateMCPRecipientPersonal(phone string) bool {
 _,isGroup,ok:=privateAPINormalizeRecipient(phone)
 return ok && !isGroup
}
