package cmd

import (
 "encoding/json"
 "strings"

 "github.com/aldinokemal/go-whatsapp-web-multidevice/config"
 "github.com/gofiber/fiber/v3"
)

// A private key can opt into the existing device-bound REST tool surface.
// This is not an administrator login: device creation/deletion, secrets,
// browser UI, MCP OAuth and CS Bridge remain outside its authority.
// Missing/unknown paths fail closed, including future REST additions.
func newPrivateLegacyToolAuth(store *privateAPIStore, backend privateAPIBackend, base string) fiber.Handler {
 root := strings.TrimRight(base, "/")
 return func(c fiber.Ctx) error {
  header := strings.TrimSpace(c.Get(fiber.HeaderAuthorization))
  if !strings.HasPrefix(header, "Bearer gwa_") { return c.Next() }
  if store == nil || backend.Resolve == nil { return privateAPIError(c,503,"PRIVATE_API_UNAVAILABLE") }
  if len(config.AppBasicAuthCredential) == 0 { return privateAPIError(c,503,"INTEGRATION_DISABLED") }
  key,valid,err := store.Authenticate(c.Context(),strings.TrimPrefix(header,"Bearer "))
  if err != nil { return privateAPIError(c,503,"KEY_STORE_UNAVAILABLE") }
  if !valid { return privateAPIError(c,401,"INVALID_API_KEY") }
  if !key.CanRESTTools { return privateAPIError(c,403,"REST_TOOLS_NOT_GRANTED") }
  route := c.Path()
  if root != "" {
   if !strings.HasPrefix(route,root+"/") { return privateAPIError(c,403,"ROUTE_NOT_ALLOWED") }
   route = strings.TrimPrefix(route,root)
  }
  if !privateLegacyRouteAllowed(c.Method(),route,key.CanGroup) {
   return privateAPIError(c,403,"ROUTE_NOT_ALLOWED")
  }

  // Each key grants exactly one canonical device, not an arbitrary alias.
  target := strings.TrimSpace(c.Get("X-Device-Id"))
  if target == "" { target = key.DeviceID }
  _, canonical, err := backend.Resolve(c.Context(),target)
  if err != nil || canonical != key.DeviceID { return privateAPIError(c,403,"DEVICE_NOT_ALLOWED") }
  if strings.HasPrefix(route,"/devices/") {
   parts:=strings.Split(strings.TrimPrefix(route,"/devices/"),"/")
   if len(parts)==0 || parts[0]=="" { return privateAPIError(c,403,"DEVICE_NOT_ALLOWED") }
   _, pathCanonical, err := backend.Resolve(c.Context(),parts[0])
   if err != nil || pathCanonical != key.DeviceID { return privateAPIError(c,403,"DEVICE_NOT_ALLOWED") }
  }
  // DeviceMiddleware uses the X-Device-Id header preferentially. Do not
  // allow query parameters to silently select another device downstream.
  c.Request().Header.Set("X-Device-Id",key.DeviceID)
  if c.Query("device_id") != "" {
   _, qCanonical, err := backend.Resolve(c.Context(),c.Query("device_id"))
   if err != nil || qCanonical != key.DeviceID { return privateAPIError(c,403,"DEVICE_NOT_ALLOWED") }
  }
  if !key.CanGroup && privateLegacyWritesToGroup(c,route) {
   return privateAPIError(c,403,"GROUP_SEND_NOT_ALLOWED")
  }
  c.Locals("private_legacy_authorized",true)
  c.Locals("private_api_key",key)
  c.Set("Cache-Control","no-store")
  return c.Next()
 }
}

func privateLegacyRouteAllowed(method, route string, allowGroup bool) bool {
 if route == "/app/info" && method=="GET" { return true }
 if strings.HasPrefix(route,"/devices/") {
  parts:=strings.Split(strings.TrimPrefix(route,"/devices/"),"/")
  // Only scoped device status/metadata; never webhook credentials or device
  // lifecycle mutations through a machine integration key.
  return method=="GET" && (len(parts)==1 || (len(parts)==2 && parts[1]=="status"))
 }
 if strings.HasPrefix(route,"/app/") {
  switch route {
  case "/app/status": return method=="GET"
  case "/app/login", "/app/login-with-code", "/app/logout", "/app/reconnect": return method=="GET"
  }
  return false
 }
 if strings.HasPrefix(route,"/send/") ||
    strings.HasPrefix(route,"/message/") ||
    strings.HasPrefix(route,"/chat/") ||
    strings.HasPrefix(route,"/user/") ||
    strings.HasPrefix(route,"/call/") {
  return true
 }
 if route=="/chats" { return method=="GET" }
 if route=="/group" || strings.HasPrefix(route,"/group/") {
  return allowGroup
 }
 if route=="/newsletter" || strings.HasPrefix(route,"/newsletter/") {
  return allowGroup
 }
 return false
}

func privateLegacyWritesToGroup(c fiber.Ctx,route string) bool {
 if c.Method() != fiber.MethodPost && c.Method() != fiber.MethodPut && c.Method() != fiber.MethodPatch {
  return false
 }
 // Only send-like routes require a recipient. Group management routes are
 // already restricted by the CanGroup capability above.
 if !strings.HasPrefix(route,"/send/") && !strings.HasPrefix(route,"/message/") {
  return false
 }
 if strings.HasPrefix(route,"/send/schedules/") { return false }
 var phone string
 kind:=strings.ToLower(strings.TrimSpace(c.Get("Content-Type")))
 if strings.HasPrefix(kind,"application/json") {
  var request struct {Phone string `json:"phone"`}
  if err:=json.Unmarshal(c.Body(),&request);err!=nil { return true }
  phone=request.Phone
 } else if strings.HasPrefix(kind,"application/x-www-form-urlencoded") ||
   strings.HasPrefix(kind,"multipart/form-data") {
  phone=c.FormValue("phone")
 } else {
  return true
 }
 // A missing recipient is only tolerated for schedule-management routes;
 // most send/message mutations require one. Deny to avoid unreviewed aliases.
 if phone=="" { return true }
 return strings.HasSuffix(phone,"@g.us") || strings.HasSuffix(phone,"@newsletter") ||
  phone=="status@broadcast" || strings.HasSuffix(phone,"@broadcast")
}

