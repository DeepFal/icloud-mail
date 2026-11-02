package httpserver

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"

	message "github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	messageMail "github.com/emersion/go-message/mail"
	"github.com/gin-gonic/gin"
	"icloud-api/internal/domain"
	"icloud-api/internal/store"
)

func (s *Server) adminAPIListWorkspaceMail(c *gin.Context) {
	c.Header("Cache-Control", "no-store")
	limit, offset, ok := adminAPIListPage(c)
	if !ok {
		return
	}
	f := domain.WorkspaceMailFilter{AdminID: mustSession(c).AdminID, Query: c.Query("query"), Folder: c.DefaultQuery("folder", "inbox"), Limit: limit, Offset: offset}
	if len(f.Query) > 256 {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "搜索内容过长")
		return
	}
	if f.Folder != "inbox" && f.Folder != "all" && f.Folder != "archived" && f.Folder != "trash" {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "文件夹无效")
		return
	}
	for _, key := range []string{"account_id", "alias_id", "group_id"} {
		if v := c.Query(key); v != "" {
			n, err := strconv.ParseInt(v, 10, 64)
			if err != nil || n < 1 {
				writeAdminAPIError(c, 400, "VALIDATION_FAILED", key+"参数无效")
				return
			}
			switch key {
			case "account_id":
				f.AccountID = n
			case "alias_id":
				f.AliasID = n
			case "group_id":
				f.GroupID = n
			}
		}
	}
	for _, key := range []string{"unread", "starred"} {
		v := c.Query(key)
		if v != "" && v != "0" && v != "1" && v != "true" && v != "false" {
			writeAdminAPIError(c, 400, "VALIDATION_FAILED", key+"参数无效")
			return
		}
		value := v == "1" || v == "true"
		if key == "unread" {
			f.Unread = value
		} else {
			f.Starred = value
		}
	}
	page, err := s.store.ListWorkspaceMail(c.Request.Context(), f)
	if err != nil {
		s.writeAdminAPIInternalError(c, err)
		return
	}
	writeAdminAPIData(c, 200, page)
}
func workspaceMailPath(c *gin.Context) (int64, uint32, bool) {
	a, e := strconv.ParseInt(c.Param("aliasID"), 10, 64)
	u, e2 := strconv.ParseUint(c.Param("uid"), 10, 32)
	if e != nil || e2 != nil || a < 1 || u < 1 {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "邮件路径参数无效")
		return 0, 0, false
	}
	return a, uint32(u), true
}
func (s *Server) workspaceMailItem(c *gin.Context) (domain.WorkspaceMailItem, bool) {
	c.Header("Cache-Control", "no-store")
	aliasID, uid, ok := workspaceMailPath(c)
	if !ok {
		return domain.WorkspaceMailItem{}, false
	}
	item, err := s.store.GetWorkspaceMail(c.Request.Context(), mustSession(c).AdminID, aliasID, uid)
	if errors.Is(err, store.ErrNotFound) {
		writeAdminAPIError(c, 404, "NOT_FOUND", "邮件不存在")
		return item, false
	}
	if err != nil {
		s.writeAdminAPIInternalError(c, err)
		return item, false
	}
	return item, true
}
func (s *Server) adminAPIPatchWorkspaceMail(c *gin.Context) {
	item, ok := s.workspaceMailItem(c)
	if !ok {
		return
	}
	var patch domain.WorkspaceMailPatch
	if !decodeAdminAPIJSON(c, &patch) {
		return
	}
	if patch.Read == nil && patch.Starred == nil && patch.Archived == nil && patch.Deleted == nil {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "请提供邮件状态")
		return
	}
	admin := mustSession(c)
	updated, err := s.store.PatchWorkspaceMail(c.Request.Context(), admin.AdminID, item.AliasID, item.MailboxUID, patch)
	if err != nil {
		s.writeAdminAPIStoreReadError(c, err)
		return
	}
	s.audit(c, &admin.AdminID, admin.Username, "workspace_mail_state", "message", fmt.Sprintf("%d/%d", item.AliasID, item.MailboxUID), "success", "")
	writeAdminAPIData(c, 200, updated)
}

type workspaceAttachment struct {
	Part         string `json:"part"`
	Name         string `json:"name"`
	ContentType  string `json:"content_type"`
	Size         int64  `json:"size"`
	DownloadPath string `json:"download_path"`
}
type workspaceMailContent struct {
	Text        string                `json:"text"`
	HTML        string                `json:"html"`
	Attachments []workspaceAttachment `json:"attachments"`
	Truncated   bool                  `json:"body_truncated"`
}

func workspaceFilename(name string) string {
	name = path.Base(strings.ReplaceAll(name, "\\", "/"))
	name = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, name)
	if name == "" || name == "." || name == "/" {
		name = "attachment"
	}
	if len(name) > 240 {
		name = "attachment"
	}
	return name
}

func parseWorkspaceMIME(reader io.Reader, aliasID int64, uid uint32, maxBytes int64) (workspaceMailContent, error) {
	out := workspaceMailContent{Attachments: []workspaceAttachment{}}
	mr, err := messageMail.CreateReader(io.LimitReader(reader, maxBytes+1))
	if err != nil && !message.IsUnknownCharset(err) {
		return out, err
	}
	if mr == nil {
		return out, errors.New("invalid MIME reader")
	}
	defer mr.Close()
	bodyBudget := int64(2 << 20)
	decodedBudget := maxBytes * 4
	for index := 1; index <= 500; index++ {
		part, err := mr.NextPart()
		if err == io.EOF {
			return out, nil
		}
		if err != nil && !message.IsUnknownCharset(err) {
			return out, err
		}
		if part == nil {
			return out, errors.New("invalid MIME part")
		}
		switch header := part.Header.(type) {
		case *messageMail.InlineHeader:
			typ, _, _ := header.ContentType()
			if typ == "text/plain" || typ == "text/html" {
				b, err := io.ReadAll(io.LimitReader(part.Body, bodyBudget+1))
				if err != nil {
					return out, err
				}
				if int64(len(b)) > bodyBudget {
					out.Truncated = true
					b = b[:bodyBudget]
				}
				bodyBudget -= int64(len(b))
				if typ == "text/plain" {
					out.Text += string(b)
				} else {
					out.HTML += string(b)
				}
			}
		case *messageMail.AttachmentHeader:
			name, _ := header.Filename()
			typ, _, _ := header.ContentType()
			if typ == "" {
				typ = "application/octet-stream"
			}
			size, err := io.Copy(io.Discard, io.LimitReader(part.Body, decodedBudget+1))
			if err != nil {
				return out, err
			}
			if size > decodedBudget {
				return out, errors.New("decoded MIME exceeds size budget")
			}
			decodedBudget -= size
			out.Attachments = append(out.Attachments, workspaceAttachment{Part: strconv.Itoa(index), Name: workspaceFilename(name), ContentType: typ, Size: size, DownloadPath: fmt.Sprintf("/mailboxes/%d/messages/%d/attachments/%d", aliasID, uid, index)})
		}
	}
	return out, errors.New("too many MIME parts")
}
func (s *Server) workspaceArchive(c *gin.Context, item domain.WorkspaceMailItem) (domain.ArchivedMailboxMessage, bool) {
	archived, err := s.store.GetArchivedMailboxMessage(c.Request.Context(), item.AliasID, item.MailboxUID)
	if err != nil {
		s.writeAdminAPIStoreReadError(c, err)
		return archived, false
	}
	return archived, true
}
func (s *Server) workspaceMIMELimit() int64 {
	if s.cfg.MaxMessageBytes > 0 {
		return min(s.cfg.MaxMessageBytes, 100<<20)
	}
	return 100 << 20
}
func (s *Server) adminAPIWorkspaceMailDetail(c *gin.Context) {
	item, ok := s.workspaceMailItem(c)
	if !ok {
		return
	}
	archived, ok := s.workspaceArchive(c, item)
	if !ok {
		return
	}
	payload := gin.H{"message": item, "text": "", "html": "", "attachments": []workspaceAttachment{}, "content_available": false, "links": []workspaceLink{}, "body_truncated": archived.BodyTruncated}
	if archived.ContentState != domain.ArchiveContentAvailable || archived.ContentBytes > s.workspaceMIMELimit() {
		writeAdminAPIData(c, 200, payload)
		return
	}
	file, err := s.store.OpenArchivedContent(archived)
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			item.ContentState = domain.ArchiveContentMissing
			payload["message"] = item
			writeAdminAPIData(c, 200, payload)
			return
		}
		s.writeAdminAPIInternalError(c, err)
		return
	}
	defer file.Close()
	content, err := parseWorkspaceMIME(file, item.AliasID, item.MailboxUID, s.workspaceMIMELimit())
	if err != nil {
		writeAdminAPIError(c, 422, "MAIL_PARSE_FAILED", "邮件原文无法解析，可以下载原文检查")
		return
	}
	payload["links"] = workspaceMailLinks(content.HTML)
	html := sanitizeWorkspaceHTML(content.HTML)
	text := content.Text
	if strings.TrimSpace(text) == "" && html != "" {
		text = plainTextFromHTML(html)
	}
	payload["text"], payload["html"], payload["attachments"], payload["content_available"], payload["body_truncated"] = text, html, content.Attachments, true, content.Truncated || archived.BodyTruncated
	writeAdminAPIData(c, 200, payload)
}
func (s *Server) adminAPIWorkspaceMailSource(c *gin.Context) {
	item, ok := s.workspaceMailItem(c)
	if !ok {
		return
	}
	archived, ok := s.workspaceArchive(c, item)
	if !ok {
		return
	}
	file, err := s.store.OpenArchivedContent(archived)
	if err != nil {
		writeAdminAPIError(c, 404, "MAIL_CONTENT_UNAVAILABLE", "邮件原文当前不可用")
		return
	}
	defer file.Close()
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fmt.Sprintf("message-%d-%d.eml", item.AliasID, item.MailboxUID)}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(200, archived.ContentBytes, "message/rfc822", file, nil)
}
func (s *Server) adminAPIWorkspaceMailAttachment(c *gin.Context) {
	item, ok := s.workspaceMailItem(c)
	if !ok {
		return
	}
	wanted, err := strconv.Atoi(c.Param("part"))
	if err != nil || wanted < 1 || wanted > 500 {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "附件编号无效")
		return
	}
	archived, ok := s.workspaceArchive(c, item)
	if !ok {
		return
	}
	if archived.ContentBytes > s.workspaceMIMELimit() {
		writeAdminAPIError(c, 413, "MAIL_TOO_LARGE", "邮件超出当前读取限制")
		return
	}
	file, err := s.store.OpenArchivedContent(archived)
	if err != nil {
		writeAdminAPIError(c, 404, "MAIL_CONTENT_UNAVAILABLE", "附件原文不可用")
		return
	}
	defer file.Close()
	mr, err := messageMail.CreateReader(io.LimitReader(file, s.workspaceMIMELimit()+1))
	if err != nil && !message.IsUnknownCharset(err) {
		writeAdminAPIError(c, 422, "MAIL_PARSE_FAILED", "附件无法解析")
		return
	}
	if mr == nil {
		writeAdminAPIError(c, 422, "MAIL_PARSE_FAILED", "附件无法解析")
		return
	}
	defer mr.Close()
	for index := 1; index <= wanted; index++ {
		part, err := mr.NextPart()
		if err != nil && !message.IsUnknownCharset(err) {
			break
		}
		if part == nil {
			break
		}
		if index != wanted {
			continue
		}
		header, ok := part.Header.(*messageMail.AttachmentHeader)
		if !ok {
			break
		}
		name, _ := header.Filename()
		c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": workspaceFilename(name)}))
		c.Header("X-Content-Type-Options", "nosniff")
		c.DataFromReader(http.StatusOK, -1, "application/octet-stream", io.LimitReader(part.Body, s.workspaceMIMELimit()*4+1), nil)
		return
	}
	writeAdminAPIError(c, 404, "NOT_FOUND", "附件不存在")
}
