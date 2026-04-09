package api

import (
	"net/http"
	"strconv"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"

	"gopodview/internal/model"
	"gopodview/internal/parser"
)

type Handler struct {
	mu       sync.RWMutex
	root     string
	fileTree *model.FileTreeNode
	pods     map[string]*model.Pod
	pp       *parser.ProjectParser
	analyzer *parser.Analyzer
}

func NewHandler(initialRoot string) *Handler {
	h := &Handler{}
	if initialRoot != "" {
		h.loadProject(initialRoot)
	}
	return h
}

func (h *Handler) loadProject(root string) error {
	tree, goFiles, err := parser.ScanProject(root)
	if err != nil {
		return err
	}

	pp := parser.NewProjectParser(root)
	analyzer := parser.NewAnalyzer(pp)
	if err := analyzer.AnalyzeAll(goFiles); err != nil {
		return err
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	h.root = root
	h.fileTree = tree
	h.pods = pp.Pods
	h.pp = pp
	h.analyzer = analyzer
	return nil
}

type SetProjectRequest struct {
	Path string `json:"path" binding:"required"`
}

func (h *Handler) SetProject(c *gin.Context) {
	var req SetProjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.loadProject(req.Path); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	h.mu.RLock()
	defer h.mu.RUnlock()
	c.JSON(http.StatusOK, gin.H{
		"message":  "project loaded",
		"path":     h.root,
		"podCount": len(h.pods),
	})
}

func (h *Handler) GetFileTree(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.fileTree == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no project loaded"})
		return
	}
	c.JSON(http.StatusOK, h.fileTree)
}

type PodEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

func (h *Handler) GetPods(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	if h.pods == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no project loaded"})
		return
	}

	var edges []PodEdge

	for _, pod := range h.pods {
		for _, dep := range pod.DependsOn {
			edges = append(edges, PodEdge{Source: pod.Path, Target: dep})
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"pods":  summarizePods(h.pods),
		"edges": edges,
	})
}

func (h *Handler) GetPod(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	path := strings.TrimPrefix(c.Param("path"), "/")
	pod, ok := h.pods[path]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "pod not found: " + path})
		return
	}

	c.JSON(http.StatusOK, pod)
}

func (h *Handler) GetContainers(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	path := strings.TrimPrefix(c.Param("path"), "/")
	pod, ok := h.pods[path]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "pod not found: " + path})
		return
	}

	c.JSON(http.StatusOK, pod.Containers)
}

func (h *Handler) GetContainer(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	fullPath := strings.TrimPrefix(c.Param("path"), "/")
	name := c.Query("name")

	pod, ok := h.pods[fullPath]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "pod not found: " + fullPath})
		return
	}

	for _, container := range pod.Containers {
		if container.Name == name {
			c.JSON(http.StatusOK, container)
			return
		}
	}

	c.JSON(http.StatusNotFound, gin.H{"error": "container not found: " + name})
}

func (h *Handler) GetDependencies(c *gin.Context) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	path := strings.TrimPrefix(c.Param("path"), "/")
	depthStr := c.DefaultQuery("depth", "1")
	depth, err := strconv.Atoi(depthStr)
	if err != nil || depth < 1 {
		depth = 1
	}
	if depth > 10 {
		depth = 10
	}

	pod, ok := h.pods[path]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "pod not found: " + path})
		return
	}

	visited := make(map[string]bool)
	depPods := make([]*model.Pod, 0)
	var edges []PodEdge

	h.collectDeps(pod, depth, visited, &depPods, &edges)

	c.JSON(http.StatusOK, gin.H{
		"root":  path,
		"depth": depth,
		"pods":  depPods,
		"edges": edges,
	})
}

func (h *Handler) GetContainerDependencies(c *gin.Context) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.pods == nil || h.analyzer == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "no project loaded"})
		return
	}

	path := strings.TrimPrefix(c.Param("path"), "/")
	name := c.Query("name")
	if name == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "missing container name"})
		return
	}

	container, pods, err := h.analyzer.LoadExternalDependenciesForContainer(path, name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	edgeSet := make(map[string]bool)
	edges := make([]PodEdge, 0, len(pods))
	for _, pod := range pods {
		key := path + "->" + pod.Path
		if edgeSet[key] {
			continue
		}
		edgeSet[key] = true
		edges = append(edges, PodEdge{Source: path, Target: pod.Path})
	}

	c.JSON(http.StatusOK, gin.H{
		"container": container,
		"pods":      summarizePodSlice(pods),
		"edges":     edges,
	})
}

func (h *Handler) collectDeps(pod *model.Pod, depth int, visited map[string]bool, pods *[]*model.Pod, edges *[]PodEdge) {
	if depth <= 0 || visited[pod.Path] {
		return
	}
	visited[pod.Path] = true
	*pods = append(*pods, pod)

	for _, depPath := range pod.DependsOn {
		*edges = append(*edges, PodEdge{Source: pod.Path, Target: depPath})
		if depPod, ok := h.pods[depPath]; ok {
			h.collectDeps(depPod, depth-1, visited, pods, edges)
		}
	}
}

func summarizePods(pods map[string]*model.Pod) []*model.Pod {
	list := make([]*model.Pod, 0, len(pods))
	for _, pod := range pods {
		list = append(list, summarizePod(pod))
	}
	return list
}

func summarizePodSlice(pods []*model.Pod) []*model.Pod {
	list := make([]*model.Pod, 0, len(pods))
	for _, pod := range pods {
		list = append(list, summarizePod(pod))
	}
	return list
}

func summarizePod(pod *model.Pod) *model.Pod {
	podCopy := *pod
	podCopy.Containers = nil
	for _, c := range pod.Containers {
		cc := *c
		cc.SourceCode = ""
		podCopy.Containers = append(podCopy.Containers, &cc)
	}
	return &podCopy
}
