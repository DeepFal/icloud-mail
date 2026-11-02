package httpserver

import "github.com/gin-gonic/gin"

func (s *Server) adminPoolReleaseReviews(c *gin.Context) {
	items, err := s.store.ListPoolReleaseReviews(c.Request.Context())
	if err != nil {
		s.poolError(c, err)
		return
	}
	c.Header("Cache-Control", "no-store")
	writeAdminAPIData(c, 200, gin.H{"items": items})
}
func (s *Server) adminResolvePoolReleaseReview(c *gin.Context) {
	var body struct {
		Unused *bool `json:"unused"`
	}
	if !decodeAdminAPIJSON(c, &body) {
		return
	}
	if body.Unused == nil {
		writeAdminAPIError(c, 400, "VALIDATION_FAILED", "请明确确认地址是否未使用")
		return
	}
	if err := s.store.ResolvePoolReleaseReview(c.Request.Context(), c.Param("leaseID"), *body.Unused); err != nil {
		s.poolError(c, err)
		return
	}
	session := mustSession(c)
	s.audit(c, &session.AdminID, session.Username, "pool_expiry_review", "pool_lease", c.Param("leaseID"), "success", "")
	writeAdminAPIData(c, 200, gin.H{"resolved": true})
}
