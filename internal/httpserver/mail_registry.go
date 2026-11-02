package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"icloud-api/internal/domain"
	"icloud-api/internal/store"
)

func (s *Server) registerMailRegistryRoutes(r *gin.RouterGroup) {
	r.GET("/services", s.listMailServices)
	r.POST("/services", s.createMailService)
	r.PATCH("/services/:id", s.updateMailService)
	r.GET("/aliases/:id/profile", s.getMailboxProfile)
	r.PATCH("/aliases/:id/profile", s.patchMailboxProfile)
	r.PUT("/aliases/:id/registrations", s.saveMailboxRegistration)
	r.DELETE("/aliases/:id/registrations/:registrationID", s.deleteMailboxRegistration)
	r.POST("/service-registrations/batch", s.batchMailboxRegistrations)
	r.GET("/service-mailboxes", s.listServiceMailboxes)
}

func registryID(c *gin.Context, name string) (int64, bool) {
	n, err := strconv.ParseInt(c.Param(name), 10, 64)
	if err != nil || n <= 0 {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "ID无效")
		return 0, false
	}
	return n, true
}
func (s *Server) registryErr(c *gin.Context, err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, store.ErrMailRegistryInput) {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "请检查服务名称、域名、登记状态和文字长度。备注最多2000字，账号标识最多200字。")
		return true
	}
	if errors.Is(err, store.ErrMailRegistryConflict) {
		writeAdminAPIError(c, 409, "CONFLICT", "保存冲突：名称可能已存在、服务已归档，或所选邮箱已有使用记录，不能登记为未注册。请核对后重试。")
		return true
	}
	if errors.Is(err, store.ErrNotFound) {
		writeAdminAPIError(c, 404, "NOT_FOUND", "资源不存在")
		return true
	}
	s.writeAdminAPIInternalError(c, err)
	return true
}
func registryBool(c *gin.Context, key string) bool {
	value := c.Query(key)
	if value != "" && value != "0" && value != "1" {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "筛选开关只接受0或1")
		return false
	}
	return true
}
func (s *Server) registryAudit(c *gin.Context, action, kind string, id int64) {
	session := mustSession(c)
	s.audit(c, &session.AdminID, session.Username, action, kind, strconv.FormatInt(id, 10), "success", "")
}
func (s *Server) listMailServices(c *gin.Context) {
	if !registryBool(c, "include_archived") {
		return
	}
	q := c.Query("query")
	if len([]rune(q)) > 200 {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "搜索内容过长")
		return
	}
	items, err := s.store.ListMailServices(c, q, c.Query("include_archived") == "1")
	if s.registryErr(c, err) {
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"items": items}})
}
func (s *Server) createMailService(c *gin.Context) {
	var in struct {
		Name   string `json:"name"`
		Domain string `json:"domain"`
		Note   string `json:"note"`
	}
	if !decodeAdminAPIJSON(c, &in) {
		return
	}
	v, err := s.store.CreateMailService(c, domain.MailService{Name: in.Name, Domain: in.Domain, Note: in.Note})
	if s.registryErr(c, err) {
		return
	}
	s.registryAudit(c, "create", "mail_service", v.ID)
	c.JSON(201, gin.H{"data": v})
}
func (s *Server) updateMailService(c *gin.Context) {
	id, ok := registryID(c, "id")
	if !ok {
		return
	}
	var in map[string]json.RawMessage
	if !decodeAdminAPIJSON(c, &in) {
		return
	}
	v, err := s.store.GetMailService(c.Request.Context(), id)
	if s.registryErr(c, err) {
		return
	}
	if len(in) == 0 {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "至少提供一个字段")
		return
	}
	for k := range in {
		if k != "name" && k != "domain" && k != "note" && k != "archived" {
			writeAdminAPIError(c, 400, "VALIDATION_FAILED", "包含未知字段")
			return
		}
	}
	var x string
	var b bool
	if raw, ok := in["name"]; ok {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &x) != nil || x == "" {
			writeAdminAPIError(c, 400, "VALIDATION_FAILED", "name无效")
			return
		}
		v.Name = x
	}
	if raw, ok := in["domain"]; ok {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &x) != nil {
			writeAdminAPIError(c, 400, "VALIDATION_FAILED", "domain无效")
			return
		}
		v.Domain = x
	}
	if raw, ok := in["note"]; ok {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &x) != nil {
			writeAdminAPIError(c, 400, "VALIDATION_FAILED", "note无效")
			return
		}
		v.Note = x
	}
	if raw, ok := in["archived"]; ok {
		if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) || json.Unmarshal(raw, &b) != nil {
			writeAdminAPIError(c, 400, "VALIDATION_FAILED", "archived无效")
			return
		}
		v.Archived = b
	}
	v, err = s.store.UpdateMailService(c, id, v)
	if s.registryErr(c, err) {
		return
	}
	c.JSON(200, gin.H{"data": v})
}
func (s *Server) getMailboxProfile(c *gin.Context) {
	id, ok := registryID(c, "id")
	if !ok {
		return
	}
	v, err := s.store.GetMailboxProfile(c, id)
	if s.registryErr(c, err) {
		return
	}
	c.JSON(200, gin.H{"data": v})
}
func (s *Server) patchMailboxProfile(c *gin.Context) {
	id, ok := registryID(c, "id")
	if !ok {
		return
	}
	var in struct {
		Note *string `json:"note"`
	}
	if !decodeAdminAPIJSON(c, &in) {
		return
	}
	if in.Note == nil {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "必须提供note")
		return
	}
	v, err := s.store.SetMailboxNote(c, id, *in.Note)
	if s.registryErr(c, err) {
		return
	}
	c.JSON(200, gin.H{"data": v})
}
func (s *Server) saveMailboxRegistration(c *gin.Context) {
	id, ok := registryID(c, "id")
	if !ok {
		return
	}
	var in domain.ServiceRegistrationInput
	if !decodeAdminAPIJSON(c, &in) {
		return
	}
	v, err := s.store.SaveMailboxRegistration(c, id, in)
	if s.registryErr(c, err) {
		return
	}
	c.JSON(200, gin.H{"data": v})
}
func (s *Server) deleteMailboxRegistration(c *gin.Context) {
	id, ok := registryID(c, "id")
	if !ok {
		return
	}
	rid, ok := registryID(c, "registrationID")
	if !ok {
		return
	}
	v, err := s.store.DeleteMailboxRegistration(c, id, rid)
	if s.registryErr(c, err) {
		return
	}
	c.JSON(200, gin.H{"data": v})
}
func (s *Server) batchMailboxRegistrations(c *gin.Context) {
	var in struct {
		AliasIDs          []int64 `json:"alias_ids"`
		ServiceID         int64   `json:"service_id"`
		AccountIdentifier string  `json:"account_identifier"`
		Status            string  `json:"status"`
		Note              string  `json:"note"`
	}
	if !decodeAdminAPIJSON(c, &in) {
		return
	}
	n, err := s.store.BatchSaveMailboxRegistrations(c, in.AliasIDs, domain.ServiceRegistrationInput{ServiceID: in.ServiceID, AccountIdentifier: in.AccountIdentifier, Status: in.Status, Note: in.Note})
	if s.registryErr(c, err) {
		return
	}
	c.JSON(200, gin.H{"data": gin.H{"updated": n}})
}
func (s *Server) listServiceMailboxes(c *gin.Context) {
	if !registryBool(c, "enabled_only") {
		return
	}
	status := strings.TrimSpace(c.Query("status"))
	switch status {
	case "", "registered", "pending", "closed", "not_registered", "unrecorded":
	default:
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "登记状态无效")
		return
	}
	parse := func(k string) (int64, bool) {
		v := c.Query(k)
		if v == "" {
			return 0, true
		}
		n, e := strconv.ParseInt(v, 10, 64)
		return n, e == nil && n > 0
	}
	sid, ok := parse("service_id")
	if !ok || sid < 1 {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "请选择服务")
		return
	}
	aid, ok := parse("account_id")
	if !ok {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "account_id无效")
		return
	}
	gid, ok := parse("group_id")
	if !ok {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "group_id无效")
		return
	}
	q := c.Query("query")
	if len([]rune(q)) > 200 {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "搜索内容过长")
		return
	}
	limit, offset := 50, 0
	var e error
	if c.Query("limit") != "" {
		limit, e = strconv.Atoi(c.Query("limit"))
	}
	if e != nil || limit < 1 || limit > 200 {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "limit无效")
		return
	}
	if c.Query("offset") != "" {
		offset, e = strconv.Atoi(c.Query("offset"))
	}
	if e != nil || offset < 0 || offset > adminAPIMaxPageOffset {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "offset无效")
		return
	}
	v, e := s.store.ListServiceMailboxes(c, domain.ServiceMailboxFilter{ServiceID: sid, AccountID: aid, GroupID: gid, Query: q, Status: status, EnabledOnly: c.Query("enabled_only") == "1", Limit: limit, Offset: offset})
	if s.registryErr(c, e) {
		return
	}
	c.JSON(200, gin.H{"data": v})
}
