package httpapi

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	cloudv1 "github.com/w7panel/w7panel-cloudconfig/api/v1alpha1"
	"github.com/w7panel/w7panel-cloudconfig/pkg/configcenter"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrlclient "sigs.k8s.io/controller-runtime/pkg/client"
)

type Server struct {
	Addr         string
	Client       ctrlclient.Client
	APIReader    ctrlclient.Reader
	FrontendRoot string
	server       *http.Server
}

func (s *Server) Start(ctx context.Context) error {
	s.server = &http.Server{Addr: s.Addr, Handler: s.router()}
	errCh := make(chan error, 1)
	go func() {
		if err := s.server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()
	select {
	case <-ctx.Done():
		return s.server.Shutdown(context.Background())
	case err := <-errCh:
		return err
	}
}

func (s *Server) NeedLeaderElection() bool {
	return false
}

func (s *Server) router() http.Handler {
	router := gin.New()
	router.Use(gin.Logger(), gin.Recovery())
	api := router.Group("/cloudconfig-api/v1")
	api.Use(s.authMiddleware())
	api.GET("/configs", s.listConfigs)
	api.POST("/configs", s.createConfig)
	api.GET("/configs/:namespace/:name", s.getConfig)
	api.PUT("/configs/:namespace/:name", s.updateConfig)
	api.DELETE("/configs/:namespace/:name", s.deleteConfig)
	api.GET("/configs/:namespace/:name/resolved", s.resolveConfig)
	api.POST("/configs/:namespace/:name/strategies/:strategy/apply", s.applyStrategy)
	api.GET("/targets", s.listTargets)
	router.NoRoute(s.serveFrontend())
	return router
}

func (s *Server) authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.GetHeader("Authorization") == "" && c.GetHeader("Authorization-ckm") == "" && c.GetHeader("X-W7Panel-Token") == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"message": "missing authorization token"})
			c.Abort()
			return
		}
		c.Next()
	}
}

func (s *Server) client() ctrlclient.Client {
	return s.Client
}

func namespaceFromQuery(c *gin.Context) string {
	if ns := strings.TrimSpace(c.Query("namespace")); ns != "" {
		return ns
	}
	if ns := strings.TrimSpace(os.Getenv("POD_NAMESPACE")); ns != "" {
		return ns
	}
	return "default"
}

func (s *Server) listConfigs(c *gin.Context) {
	list := &cloudv1.CloudConfigList{}
	if err := s.client().List(c.Request.Context(), list, ctrlclient.InNamespace(namespaceFromQuery(c))); err != nil {
		errorJSON(c, err)
		return
	}
	c.JSON(http.StatusOK, list.Items)
}

func (s *Server) createConfig(c *gin.Context) {
	cfg := &cloudv1.CloudConfig{}
	if err := c.ShouldBindJSON(cfg); err != nil {
		errorJSON(c, err)
		return
	}
	if cfg.Namespace == "" {
		cfg.Namespace = namespaceFromQuery(c)
	}
	if cfg.Name == "" {
		cfg.Name = generatedName(cfg.Spec.Name)
	}
	cfg.TypeMeta = metav1.TypeMeta{APIVersion: cloudv1.GroupVersion.String(), Kind: cloudv1.CloudConfigKind}
	configcenter.NormalizeConfig(cfg)
	if err := configcenter.Validate(cfg); err != nil {
		errorJSON(c, err)
		return
	}
	now := metav1.Now()
	cfg.Status.CreatedAt = now
	cfg.Status.UpdatedAt = now
	cfg.Status.Revision = configcenter.Revision(cfg)
	if err := s.client().Create(c.Request.Context(), cfg); err != nil {
		errorJSON(c, err)
		return
	}
	_ = s.client().Status().Update(c.Request.Context(), cfg)
	c.JSON(http.StatusCreated, cfg)
}

func (s *Server) getConfig(c *gin.Context) {
	cfg, ok := s.loadConfig(c, c.Param("namespace"), c.Param("name"))
	if !ok {
		return
	}
	c.JSON(http.StatusOK, cfg)
}

func (s *Server) updateConfig(c *gin.Context) {
	current, ok := s.loadConfig(c, c.Param("namespace"), c.Param("name"))
	if !ok {
		return
	}
	var payload cloudv1.CloudConfig
	if err := c.ShouldBindJSON(&payload); err != nil {
		errorJSON(c, err)
		return
	}
	current.Spec = payload.Spec
	configcenter.NormalizeConfig(current)
	if err := configcenter.Validate(current); err != nil {
		errorJSON(c, err)
		return
	}
	if err := s.client().Update(c.Request.Context(), current); err != nil {
		errorJSON(c, err)
		return
	}
	c.JSON(http.StatusOK, current)
}

func (s *Server) deleteConfig(c *gin.Context) {
	cfg, ok := s.loadConfig(c, c.Param("namespace"), c.Param("name"))
	if !ok {
		return
	}
	if err := s.client().Delete(c.Request.Context(), cfg); err != nil {
		errorJSON(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

func (s *Server) resolveConfig(c *gin.Context) {
	cfg, ok := s.loadConfig(c, c.Param("namespace"), c.Param("name"))
	if !ok {
		return
	}
	items, err := configcenter.ResolveItems(cfg, s.lookupConfig(c.Request.Context()), c.Query("version"))
	if err != nil {
		errorJSON(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items, "data": configcenter.ItemsToData(items)})
}

type applyRequest struct {
	Version    string `json:"version"`
	AutoDeploy *bool `json:"autoDeploy"`
}

func (s *Server) applyStrategy(c *gin.Context) {
	cfg, ok := s.loadConfig(c, c.Param("namespace"), c.Param("name"))
	if !ok {
		return
	}
	var req applyRequest
	_ = c.ShouldBindJSON(&req)
	index := -1
	for i, strategy := range cfg.Spec.Strategies {
		if strategy.ID == c.Param("strategy") {
			index = i
			break
		}
	}
	if index < 0 {
		c.JSON(http.StatusNotFound, gin.H{"message": "strategy not found"})
		return
	}
	strategy := cfg.Spec.Strategies[index]
	if req.AutoDeploy != nil {
		cfg.Spec.Strategies[index].AutoDeploy = *req.AutoDeploy
		strategy.AutoDeploy = *req.AutoDeploy
	}
	if req.Version != "" || strategy.LastSelectedVersion != req.Version {
		cfg.Spec.Strategies[index].LastSelectedVersion = req.Version
		strategy.LastSelectedVersion = req.Version
	}
	result, err := configcenter.ApplyStrategy(c.Request.Context(), s.client(), cfg, s.lookupConfig(c.Request.Context()), strategy, req.Version)
	if err != nil {
		errorJSON(c, err)
		return
	}
	now := metav1.Now()
	cfg.Status.LastApplied = upsertApplyStatus(cfg.Status.LastApplied, cloudv1.ApplyStatus{
		StrategyID: strategy.ID,
		Version:    req.Version,
		Revision:   result.Revision,
		AppliedAt:  now,
		Success:    true,
	})
	_ = s.client().Update(c.Request.Context(), cfg)
	_ = s.client().Status().Update(c.Request.Context(), cfg)
	c.JSON(http.StatusOK, result)
}

func (s *Server) listTargets(c *gin.Context) {
	namespace := namespaceFromQuery(c)
	targets := []gin.H{}
	addPods := func(kind string, items any) {
		switch list := items.(type) {
		case []appsv1.Deployment:
			for _, item := range list {
				targets = append(targets, targetFromTemplate(namespace, kind, item.Name, item.Labels["w7.cc/app-group"], item.Spec.Template.Spec.Containers))
			}
		case []appsv1.StatefulSet:
			for _, item := range list {
				targets = append(targets, targetFromTemplate(namespace, kind, item.Name, item.Labels["w7.cc/app-group"], item.Spec.Template.Spec.Containers))
			}
		case []appsv1.DaemonSet:
			for _, item := range list {
				targets = append(targets, targetFromTemplate(namespace, kind, item.Name, item.Labels["w7.cc/app-group"], item.Spec.Template.Spec.Containers))
			}
		}
	}
	deployments := &appsv1.DeploymentList{}
	if err := s.client().List(c.Request.Context(), deployments, ctrlclient.InNamespace(namespace)); err != nil {
		errorJSON(c, err)
		return
	}
	addPods("Deployment", deployments.Items)
	statefulSets := &appsv1.StatefulSetList{}
	if err := s.client().List(c.Request.Context(), statefulSets, ctrlclient.InNamespace(namespace)); err != nil {
		errorJSON(c, err)
		return
	}
	addPods("StatefulSet", statefulSets.Items)
	daemonSets := &appsv1.DaemonSetList{}
	if err := s.client().List(c.Request.Context(), daemonSets, ctrlclient.InNamespace(namespace)); err != nil {
		errorJSON(c, err)
		return
	}
	addPods("DaemonSet", daemonSets.Items)
	c.JSON(http.StatusOK, targets)
}

func targetFromTemplate(namespace, kind, name, group string, containers []corev1.Container) gin.H {
	names := make([]string, 0, len(containers))
	for _, container := range containers {
		names = append(names, container.Name)
	}
	return gin.H{"namespace": namespace, "kind": kind, "name": name, "group": group, "containers": names}
}

func (s *Server) loadConfig(c *gin.Context, namespace, name string) (*cloudv1.CloudConfig, bool) {
	cfg := &cloudv1.CloudConfig{}
	if err := s.client().Get(c.Request.Context(), types.NamespacedName{Namespace: namespace, Name: name}, cfg); err != nil {
		errorJSON(c, err)
		return nil, false
	}
	return cfg, true
}

func (s *Server) lookupConfig(ctx context.Context) func(namespace, name string) (*cloudv1.CloudConfig, bool) {
	return func(namespace, name string) (*cloudv1.CloudConfig, bool) {
		cfg := &cloudv1.CloudConfig{}
		if err := s.client().Get(ctx, types.NamespacedName{Namespace: namespace, Name: name}, cfg); err != nil {
			return nil, false
		}
		return cfg, true
	}
}

func upsertApplyStatus(list []cloudv1.ApplyStatus, item cloudv1.ApplyStatus) []cloudv1.ApplyStatus {
	for i := range list {
		if list[i].StrategyID == item.StrategyID {
			list[i] = item
			return list
		}
	}
	return append(list, item)
}

func generatedName(title string) string {
	value := strings.ToLower(strings.TrimSpace(title))
	value = regexp.MustCompile(`[^a-z0-9-]+`).ReplaceAllString(value, "-")
	value = strings.Trim(value, "-")
	if value == "" {
		value = "config"
	}
	if len(value) > 36 {
		value = strings.Trim(value[:36], "-")
	}
	return "cloudconfig-" + value + "-" + strconv.FormatInt(time.Now().UnixNano()%1000000, 10)
}

func errorJSON(c *gin.Context, err error) {
	status := http.StatusBadRequest
	c.JSON(status, gin.H{"message": err.Error()})
}

func (s *Server) serveFrontend() gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			c.Status(http.StatusNotFound)
			return
		}
		if strings.HasPrefix(c.Request.URL.Path, "/cloudconfig-api/") {
			c.Status(http.StatusNotFound)
			return
		}
		root := strings.TrimSpace(s.FrontendRoot)
		if root == "" {
			root = strings.TrimSpace(os.Getenv("KO_DATA_PATH"))
		}
		if root == "" {
			root = "kodata"
		}
		rootFS := os.DirFS(root)
		requestPath := strings.TrimPrefix(path.Clean("/"+c.Request.URL.Path), "/")
		if requestPath == "" || requestPath == "." {
			requestPath = "index.html"
		}
		if hasFile(rootFS, requestPath) {
			http.FileServer(http.FS(rootFS)).ServeHTTP(c.Writer, c.Request)
			return
		}
		if strings.Contains(path.Base(requestPath), ".") {
			c.Status(http.StatusNotFound)
			return
		}
		content, err := fs.ReadFile(rootFS, "index.html")
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", content)
	}
}

func hasFile(rootFS fs.FS, name string) bool {
	info, err := fs.Stat(rootFS, name)
	return err == nil && !info.IsDir()
}

var _ = time.Second
