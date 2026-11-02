package httpserver

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"icloud-api/internal/domain"
	"icloud-api/internal/store"
)

func (s *Server) SetHistoryTaskAvailable(enabled bool) { s.historyTaskAvailable = enabled }
func (s *Server) adminAPIGetMailPreferences(c *gin.Context) {
	id, ok := adminAPIParseID(c)
	if !ok {
		return
	}
	p, err := s.store.GetMailboxPreferences(c.Request.Context(), id)
	if err != nil {
		s.writeAdminAPIStoreReadError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	writeAdminAPIData(c, 200, p)
}
func (s *Server) adminAPISetMailPreferences(c *gin.Context) {
	id, ok := adminAPIParseID(c)
	if !ok {
		return
	}
	var body struct {
		BackgroundEnabled *bool `json:"background_enabled"`
		HistoryDays       *int  `json:"history_days"`
	}
	if !decodeAdminAPIJSON(c, &body) {
		return
	}
	if body.BackgroundEnabled == nil || body.HistoryDays == nil || *body.HistoryDays < 1 || *body.HistoryDays > 3650 {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "请提供后台收件开关与1至3650天的补取范围")
		return
	}
	pref, err := s.store.SetMailboxPreferences(c.Request.Context(), domain.MailboxPreferences{AccountID: id, BackgroundEnabled: *body.BackgroundEnabled, HistoryDays: *body.HistoryDays})
	if err != nil {
		s.writeAdminAPIStoreReadError(c, err)
		return
	}
	session := mustSession(c)
	s.audit(c, &session.AdminID, session.Username, "mail_preferences", "account", strconv.FormatInt(id, 10), "success", "")
	writeAdminAPIData(c, 200, pref)
}
func (s *Server) adminAPIGetHistoryJob(c *gin.Context) {
	id, ok := adminAPIParseID(c)
	if !ok {
		return
	}
	if _, err := s.store.GetAccount(c.Request.Context(), id); err != nil {
		s.writeAdminAPIStoreReadError(c, err)
		return
	}
	j, err := s.store.LatestMailHistoryJob(c.Request.Context(), id)
	if errors.Is(err, store.ErrNotFound) {
		writeAdminAPIData(c, 200, gin.H{"job": nil})
		return
	}
	if err != nil {
		s.writeAdminAPIInternalError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	writeAdminAPIData(c, 200, gin.H{"job": j})
}
func (s *Server) adminAPIStartHistoryJob(c *gin.Context) {
	id, ok := adminAPIParseID(c)
	if !ok {
		return
	}
	if !s.historyTaskAvailable {
		writeAdminAPIError(c, 503, "HISTORY_WORKER_UNAVAILABLE", "历史补取服务尚未就绪")
		return
	}
	var body struct {
		OperationID string `json:"operation_id"`
		Days        int    `json:"days"`
	}
	if !decodeAdminAPIJSON(c, &body) {
		return
	}
	account, err := s.store.GetAccount(c.Request.Context(), id)
	if err != nil {
		s.writeAdminAPIStoreReadError(c, err)
		return
	}
	if !account.Enabled {
		writeAdminAPIError(c, 409, "ACCOUNT_DISABLED", "请先启用主号")
		return
	}
	transport, err := s.store.GetAccountMailTransport(c.Request.Context(), id)
	if err != nil {
		s.writeAdminAPIInternalError(c, err)
		return
	}
	if transport != "imap" {
		writeAdminAPIError(c, 409, "IMAP_REQUIRED", "历史补取需要配置 IMAP 收件连接")
		return
	}
	j, err := s.store.CreateMailHistoryJob(c.Request.Context(), account, body.OperationID, body.Days)
	if errors.Is(err, store.ErrMailHistoryInput) {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "请求编号或补取天数无效")
		return
	}
	if errors.Is(err, store.ErrMailHistoryConflict) {
		writeAdminAPIError(c, 409, "HISTORY_JOB_CONFLICT", "已有未完成任务或请求编号已被使用，请刷新任务状态")
		return
	}
	if err != nil {
		s.writeAdminAPIInternalError(c, err)
		return
	}
	session := mustSession(c)
	s.audit(c, &session.AdminID, session.Username, "history_mail_start", "account", strconv.FormatInt(id, 10), "success", "")
	writeAdminAPIData(c, http.StatusAccepted, j)
}
func (s *Server) adminAPIActHistoryJob(c *gin.Context) {
	id, ok := adminAPIParseID(c)
	if !ok {
		return
	}
	action := c.Param("action")
	if action != "pause" && action != "resume" && action != "cancel" {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "任务操作无效")
		return
	}
	if action == "resume" {
		account, err := s.store.GetAccount(c.Request.Context(), id)
		if err != nil {
			s.writeAdminAPIStoreReadError(c, err)
			return
		}
		job, err := s.store.GetMailHistoryJob(c.Request.Context(), id, c.Param("jobID"))
		if err != nil {
			s.writeAdminAPIStoreReadError(c, err)
			return
		}
		if !account.Enabled || account.MailboxConfigVersion != job.ConfigVersion || account.MailTransportVersion != job.TransportVersion {
			writeAdminAPIError(c, 409, "HISTORY_SOURCE_CHANGED", "邮箱配置已变化，请取消原任务并新建补取")
			return
		}
	}
	j, err := s.store.ActMailHistoryJob(c.Request.Context(), id, c.Param("jobID"), action)
	if errors.Is(err, store.ErrMailHistoryConflict) {
		writeAdminAPIError(c, 409, "HISTORY_JOB_CONFLICT", "任务状态已变化，请刷新")
		return
	}
	if err != nil {
		s.writeAdminAPIStoreReadError(c, err)
		return
	}
	session := mustSession(c)
	s.audit(c, &session.AdminID, session.Username, "history_mail_"+action, "account", strconv.FormatInt(id, 10), "success", "")
	writeAdminAPIData(c, 200, j)
}
func (s *Server) adminAPIWorkspaceTasks(c *gin.Context) {
	accounts, err := s.store.ListAccounts(c.Request.Context())
	if err != nil {
		s.writeAdminAPIInternalError(c, err)
		return
	}
	names := map[int64]string{}
	for _, a := range accounts {
		names[a.ID] = a.Name
	}
	history, err := s.store.ListMailHistoryJobs(c.Request.Context(), 100)
	if err != nil {
		s.writeAdminAPIInternalError(c, err)
		return
	}
	type historyDTO struct {
		domain.MailHistoryJob
		AccountName string `json:"account_name"`
	}
	historyItems := []historyDTO{}
	for _, j := range history {
		historyItems = append(historyItems, historyDTO{j, names[j.AccountID]})
	}
	type creationDTO struct {
		store.AliasCreationJob
		AccountName string `json:"account_name"`
	}
	creationItems := []creationDTO{}
	for _, a := range accounts {
		j, e := s.store.GetLatestAliasCreationJob(c.Request.Context(), a.ID)
		if e == nil {
			creationItems = append(creationItems, creationDTO{j, a.Name})
		} else if !errors.Is(e, store.ErrNotFound) {
			s.writeAdminAPIInternalError(c, e)
			return
		}
	}
	deletionItems := []adminAPIAliasDeletionJobDTO{}
	j, err := s.store.GetLatestAliasDeletionJob(c.Request.Context(), mustSession(c).AdminID)
	if err == nil {
		deletionItems = append(deletionItems, s.adminAPIAliasDeletionJobSnapshot(j))
	} else if !errors.Is(err, store.ErrNotFound) {
		s.writeAdminAPIInternalError(c, err)
		return
	}
	automaticPlans := []gin.H{}
	for _, a := range accounts {
		plan, e := s.store.GetAliasCreationSchedule(c.Request.Context(), a.ID)
		if e == nil && plan.Enabled {
			automaticPlans = append(automaticPlans, gin.H{"id": "auto-" + strconv.FormatInt(a.ID, 10), "account_id": a.ID, "account_name": a.Name, "status": "waiting", "enabled": plan.Enabled, "next_run_at": plan.NextRunAt, "last_error": plan.LastError, "last_created_at": plan.LastCreatedAt, "last_alias_address": plan.LastAliasAddress})
		} else if e != nil && !errors.Is(e, store.ErrNotFound) {
			s.writeAdminAPIInternalError(c, e)
			return
		}
	}
	c.Header("Cache-Control", "no-store")
	writeAdminAPIData(c, 200, gin.H{"history_jobs": historyItems, "creation_jobs": creationItems, "deletion_jobs": deletionItems, "automatic_plans": automaticPlans})
}

// Selectors intentionally exclude decrypted mailbox credentials.
func (s *Server) adminAPIWorkspaceOptions(c *gin.Context) {
	accounts, err := s.store.ListAccounts(c.Request.Context())
	if err != nil {
		s.writeAdminAPIInternalError(c, err)
		return
	}
	aliases, err := s.store.ListAliases(c.Request.Context())
	if err != nil {
		s.writeAdminAPIInternalError(c, err)
		return
	}
	groups, err := s.store.ListMailGroups(c.Request.Context())
	if err != nil {
		s.writeAdminAPIInternalError(c, err)
		return
	}
	ac := []gin.H{}
	al := []gin.H{}
	gr := []gin.H{}
	for _, a := range accounts {
		ac = append(ac, gin.H{"id": a.ID, "name": a.Name, "email": a.Email, "enabled": a.Enabled})
	}
	for _, a := range aliases {
		al = append(al, gin.H{"id": a.ID, "accountId": a.AccountID, "address": a.Address, "label": a.Label, "note": a.Note, "groupId": a.GroupID})
	}
	for _, g := range groups {
		gr = append(gr, gin.H{"id": g.ID, "name": g.Name})
	}
	c.Header("Cache-Control", "no-store")
	writeAdminAPIData(c, 200, gin.H{"accounts": ac, "aliases": al, "groups": gr})
}
