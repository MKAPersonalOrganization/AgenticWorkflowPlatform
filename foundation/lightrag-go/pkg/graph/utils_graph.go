// Package graph provides graph manipulation utilities for LightRAG.
// It includes functions for managing entities and relations in the knowledge graph,
// including creation, deletion, editing, and merging operations.
package graph

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/sirupsen/logrus"
)

// GraphFieldSep is the separator used for multi-value fields in the graph.
const GraphFieldSep = "<SEP>"

// Logger for the graph package.
var logger = logrus.New()

// DeletionResult represents the result of a deletion operation.
type DeletionResult struct {
	Status     string `json:"status"`      // "success", "not_found", "fail"
	DocID      string `json:"doc_id"`      // ID or name of the deleted item
	Message    string `json:"message"`     // Human-readable message
	StatusCode int    `json:"status_code"` // HTTP-like status code
}

// OperationSummary contains metadata about an entity edit operation.
type OperationSummary struct {
	Merged          bool    `json:"merged"`
	MergeStatus     string  `json:"merge_status"`     // "success", "failed", "not_attempted"
	MergeError      *string `json:"merge_error"`      // Error message if merge failed
	OperationStatus string  `json:"operation_status"` // "success", "partial_success", "failure"
	TargetEntity    *string `json:"target_entity"`    // Target entity name if renaming/merging
	FinalEntity     string  `json:"final_entity"`     // Final entity name after operation
	Renamed         bool    `json:"renamed"`          // Whether entity was renamed
}

// EntityInfo contains detailed information about an entity.
type EntityInfo struct {
	EntityName       string                 `json:"entity_name"`
	SourceID         string                 `json:"source_id,omitempty"`
	GraphData        map[string]interface{} `json:"graph_data,omitempty"`
	VectorData       map[string]interface{} `json:"vector_data,omitempty"`
	OperationSummary *OperationSummary      `json:"operation_summary,omitempty"`
}

// RelationInfo contains detailed information about a relation.
type RelationInfo struct {
	SrcEntity  string                 `json:"src_entity"`
	TgtEntity  string                 `json:"tgt_entity"`
	SourceID   string                 `json:"source_id,omitempty"`
	GraphData  map[string]interface{} `json:"graph_data,omitempty"`
	VectorData map[string]interface{} `json:"vector_data,omitempty"`
}

// ChunkTrackingData represents data stored for tracking chunk references.
type ChunkTrackingData struct {
	ChunkIDs []string `json:"chunk_ids"`
	Count    int      `json:"count"`
}

// MergeStrategy defines the strategy for merging field values.
type MergeStrategy string

const (
	MergeStrategyConcatenate     MergeStrategy = "concatenate"
	MergeStrategyKeepFirst       MergeStrategy = "keep_first"
	MergeStrategyKeepLast        MergeStrategy = "keep_last"
	MergeStrategyJoinUnique      MergeStrategy = "join_unique"
	MergeStrategyJoinUniqueComma MergeStrategy = "join_unique_comma"
	MergeStrategyMax             MergeStrategy = "max"
)

// BaseKVStorage interface for key-value storage operations.
type BaseKVStorage interface {
	GetByID(id string) (map[string]interface{}, error)
	GetByIDs(ids []string) (map[string]map[string]interface{}, error)
	Upsert(data map[string]interface{}) error
	Delete(ids []string) error
	IndexDoneCallback() error
}

// BaseVectorStorage interface for vector database operations.
type BaseVectorStorage interface {
	GetByID(id string) (map[string]interface{}, error)
	Upsert(data map[string]interface{}) error
	Delete(ids []string) error
	DeleteEntity(entityName string) error
	DeleteEntityRelation(entityName string) error
	IndexDoneCallback() error
	GetGlobalConfig() map[string]interface{}
}

// BaseGraphStorage interface for graph storage operations.
type BaseGraphStorage interface {
	HasNode(nodeName string) (bool, error)
	HasEdge(srcName, tgtName string) (bool, error)
	GetNode(nodeName string) (map[string]interface{}, error)
	GetEdge(srcName, tgtName string) (map[string]interface{}, error)
	GetNodeEdges(nodeName string) ([][2]string, error)
	UpsertNode(nodeName string, nodeData map[string]interface{}) error
	UpsertEdge(srcName, tgtName string, edgeData map[string]interface{}) error
	DeleteNode(nodeName string) error
	RemoveEdges(edges [][2]string) error
	IndexDoneCallback() error
}

// GraphOperator provides graph manipulation operations.
type GraphOperator struct {
	Graph                 BaseGraphStorage
	EntitiesVDB           BaseVectorStorage
	RelationshipsVDB      BaseVectorStorage
	EntityChunksStorage   BaseKVStorage
	RelationChunksStorage BaseKVStorage
	Workspace             string
	mu                    sync.Mutex
	keyedLocks            map[string]*sync.Mutex
}

// NewGraphOperator creates a new GraphOperator instance.
func NewGraphOperator(
	graph BaseGraphStorage,
	entitiesVDB BaseVectorStorage,
	relationshipsVDB BaseVectorStorage,
	entityChunksStorage BaseKVStorage,
	relationChunksStorage BaseKVStorage,
	workspace string,
) *GraphOperator {
	return &GraphOperator{
		Graph:                 graph,
		EntitiesVDB:           entitiesVDB,
		RelationshipsVDB:      relationshipsVDB,
		EntityChunksStorage:   entityChunksStorage,
		RelationChunksStorage: relationChunksStorage,
		Workspace:             workspace,
		keyedLocks:            make(map[string]*sync.Mutex),
	}
}

// ComputeMDHashID computes an MD5 hash ID with an optional prefix.
func ComputeMDHashID(content, prefix string) string {
	hash := md5.Sum([]byte(content))
	hashStr := hex.EncodeToString(hash[:])
	if prefix != "" {
		return prefix + hashStr
	}
	return hashStr
}

// MakeRelationChunkKey creates a storage key for relation chunk tracking.
func MakeRelationChunkKey(src, tgt string) string {
	// Normalize entity order for consistent key generation
	if src > tgt {
		src, tgt = tgt, src
	}
	return fmt.Sprintf("%s|%s", src, tgt)
}

// getKeyedLock returns a mutex for the given keys (sorted and joined).
func (g *GraphOperator) getKeyedLock(keys []string) *sync.Mutex {
	sortedKeys := make([]string, len(keys))
	copy(sortedKeys, keys)
	sort.Strings(sortedKeys)
	lockKey := strings.Join(sortedKeys, "|")

	g.mu.Lock()
	defer g.mu.Unlock()

	if lock, exists := g.keyedLocks[lockKey]; exists {
		return lock
	}
	lock := &sync.Mutex{}
	g.keyedLocks[lockKey] = lock
	return lock
}

// persistGraphUpdates persists updates after graph operations.
func (g *GraphOperator) persistGraphUpdates() error {
	var errs []error

	if g.EntitiesVDB != nil {
		if err := g.EntitiesVDB.IndexDoneCallback(); err != nil {
			errs = append(errs, fmt.Errorf("entities VDB persist: %w", err))
		}
	}
	if g.RelationshipsVDB != nil {
		if err := g.RelationshipsVDB.IndexDoneCallback(); err != nil {
			errs = append(errs, fmt.Errorf("relationships VDB persist: %w", err))
		}
	}
	if g.Graph != nil {
		if err := g.Graph.IndexDoneCallback(); err != nil {
			errs = append(errs, fmt.Errorf("graph persist: %w", err))
		}
	}
	if g.EntityChunksStorage != nil {
		if err := g.EntityChunksStorage.IndexDoneCallback(); err != nil {
			errs = append(errs, fmt.Errorf("entity chunks storage persist: %w", err))
		}
	}
	if g.RelationChunksStorage != nil {
		if err := g.RelationChunksStorage.IndexDoneCallback(); err != nil {
			errs = append(errs, fmt.Errorf("relation chunks storage persist: %w", err))
		}
	}

	if len(errs) > 0 {
		errMsgs := make([]string, len(errs))
		for i, err := range errs {
			errMsgs[i] = err.Error()
		}
		return errors.New(strings.Join(errMsgs, "; "))
	}
	return nil
}

// DeleteByEntity deletes an entity and all its relationships.
func (g *GraphOperator) DeleteByEntity(entityName string) DeletionResult {
	lock := g.getKeyedLock([]string{entityName})
	lock.Lock()
	defer lock.Unlock()

	// Check if the entity exists
	exists, err := g.Graph.HasNode(entityName)
	if err != nil {
		return DeletionResult{
			Status:     "fail",
			DocID:      entityName,
			Message:    fmt.Sprintf("Error checking entity '%s': %v", entityName, err),
			StatusCode: 500,
		}
	}
	if !exists {
		logger.Warnf("Entity '%s' not found.", entityName)
		return DeletionResult{
			Status:     "not_found",
			DocID:      entityName,
			Message:    fmt.Sprintf("Entity '%s' not found.", entityName),
			StatusCode: 404,
		}
	}

	// Retrieve related relationships before deleting the node
	edges, err := g.Graph.GetNodeEdges(entityName)
	if err != nil {
		return DeletionResult{
			Status:     "fail",
			DocID:      entityName,
			Message:    fmt.Sprintf("Error getting edges for '%s': %v", entityName, err),
			StatusCode: 500,
		}
	}
	relatedRelationsCount := len(edges)

	// Clean up chunk tracking storages before deletion
	if g.EntityChunksStorage != nil {
		if err := g.EntityChunksStorage.Delete([]string{entityName}); err != nil {
			logger.Warnf("Failed to delete entity chunk tracking for '%s': %v", entityName, err)
		} else {
			logger.Infof("Entity Delete: removed chunk tracking for `%s`", entityName)
		}
	}

	if g.RelationChunksStorage != nil && len(edges) > 0 {
		relationKeysToDelete := make([]string, 0, len(edges))
		for _, edge := range edges {
			storageKey := MakeRelationChunkKey(edge[0], edge[1])
			relationKeysToDelete = append(relationKeysToDelete, storageKey)
		}
		if len(relationKeysToDelete) > 0 {
			if err := g.RelationChunksStorage.Delete(relationKeysToDelete); err != nil {
				logger.Warnf("Failed to delete relation chunk tracking: %v", err)
			} else {
				logger.Infof("Entity Delete: removed chunk tracking for %d relations", len(relationKeysToDelete))
			}
		}
	}

	// Delete from vector databases
	if err := g.EntitiesVDB.DeleteEntity(entityName); err != nil {
		logger.Warnf("Failed to delete entity from VDB: %v", err)
	}
	if err := g.RelationshipsVDB.DeleteEntityRelation(entityName); err != nil {
		logger.Warnf("Failed to delete entity relations from VDB: %v", err)
	}

	// Delete from graph
	if err := g.Graph.DeleteNode(entityName); err != nil {
		return DeletionResult{
			Status:     "fail",
			DocID:      entityName,
			Message:    fmt.Sprintf("Error deleting entity '%s' from graph: %v", entityName, err),
			StatusCode: 500,
		}
	}

	message := fmt.Sprintf("Entity Delete: remove '%s' and its %d relations", entityName, relatedRelationsCount)
	logger.Info(message)

	if err := g.persistGraphUpdates(); err != nil {
		logger.Warnf("Failed to persist graph updates: %v", err)
	}

	return DeletionResult{
		Status:     "success",
		DocID:      entityName,
		Message:    message,
		StatusCode: 200,
	}
}

// DeleteByRelation deletes a relation between two entities.
func (g *GraphOperator) DeleteByRelation(sourceEntity, targetEntity string) DeletionResult {
	relationStr := fmt.Sprintf("%s -> %s", sourceEntity, targetEntity)

	// Normalize entity order for undirected graph
	if sourceEntity > targetEntity {
		sourceEntity, targetEntity = targetEntity, sourceEntity
	}

	lock := g.getKeyedLock([]string{sourceEntity, targetEntity})
	lock.Lock()
	defer lock.Unlock()

	// Check if the relation exists
	edgeExists, err := g.Graph.HasEdge(sourceEntity, targetEntity)
	if err != nil {
		return DeletionResult{
			Status:     "fail",
			DocID:      relationStr,
			Message:    fmt.Sprintf("Error checking relation: %v", err),
			StatusCode: 500,
		}
	}
	if !edgeExists {
		message := fmt.Sprintf("Relation from '%s' to '%s' does not exist", sourceEntity, targetEntity)
		logger.Warn(message)
		return DeletionResult{
			Status:     "not_found",
			DocID:      relationStr,
			Message:    message,
			StatusCode: 404,
		}
	}

	// Clean up chunk tracking storage before deletion
	if g.RelationChunksStorage != nil {
		storageKey := MakeRelationChunkKey(sourceEntity, targetEntity)
		if err := g.RelationChunksStorage.Delete([]string{storageKey}); err != nil {
			logger.Warnf("Failed to delete relation chunk tracking: %v", err)
		} else {
			logger.Infof("Relation Delete: removed chunk tracking for `%s`~`%s`", sourceEntity, targetEntity)
		}
	}

	// Delete relation from vector database
	relIDsToDelete := []string{
		ComputeMDHashID(sourceEntity+targetEntity, "rel-"),
		ComputeMDHashID(targetEntity+sourceEntity, "rel-"),
	}
	if err := g.RelationshipsVDB.Delete(relIDsToDelete); err != nil {
		logger.Warnf("Failed to delete relation from VDB: %v", err)
	}

	// Delete relation from knowledge graph
	if err := g.Graph.RemoveEdges([][2]string{{sourceEntity, targetEntity}}); err != nil {
		return DeletionResult{
			Status:     "fail",
			DocID:      relationStr,
			Message:    fmt.Sprintf("Error deleting relation: %v", err),
			StatusCode: 500,
		}
	}

	message := fmt.Sprintf("Relation Delete: `%s`~`%s` deleted successfully", sourceEntity, targetEntity)
	logger.Info(message)

	if err := g.persistGraphUpdates(); err != nil {
		logger.Warnf("Failed to persist graph updates: %v", err)
	}

	return DeletionResult{
		Status:     "success",
		DocID:      relationStr,
		Message:    message,
		StatusCode: 200,
	}
}

// EditEntity edits entity information.
func (g *GraphOperator) EditEntity(entityName string, updatedData map[string]string, allowRename, allowMerge bool) (*EntityInfo, error) {
	newEntityName := entityName
	if name, ok := updatedData["entity_name"]; ok && name != "" {
		newEntityName = name
	}
	isRenaming := newEntityName != entityName

	lockKeys := []string{entityName}
	if isRenaming {
		lockKeys = append(lockKeys, newEntityName)
	}

	lock := g.getKeyedLock(lockKeys)
	lock.Lock()
	defer lock.Unlock()

	operationSummary := &OperationSummary{
		Merged:          false,
		MergeStatus:     "not_attempted",
		MergeError:      nil,
		OperationStatus: "success",
		TargetEntity:    nil,
		FinalEntity:     newEntityName,
		Renamed:         isRenaming,
	}

	if isRenaming && !allowRename {
		return nil, errors.New("entity renaming is not allowed. Set allowRename=true to enable this feature")
	}

	if isRenaming {
		targetExists, err := g.Graph.HasNode(newEntityName)
		if err != nil {
			return nil, fmt.Errorf("error checking target entity: %w", err)
		}

		if targetExists {
			if !allowMerge {
				return nil, fmt.Errorf("entity name '%s' already exists, cannot rename", newEntityName)
			}

			logger.Infof("Entity Edit: `%s` will be merged into `%s`", entityName, newEntityName)

			// Separate non-name updates
			nonNameUpdates := make(map[string]string)
			for k, v := range updatedData {
				if k != "entity_name" {
					nonNameUpdates[k] = v
				}
			}

			// Apply non-name updates first if any
			nonNameUpdatesApplied := false
			if len(nonNameUpdates) > 0 {
				logger.Info("Entity Edit: applying non-name updates before merge")
				if err := g.editEntityImpl(entityName, nonNameUpdates); err != nil {
					logger.Errorf("Entity Edit: non-name updates failed: %v", err)
					return nil, err
				}
				nonNameUpdatesApplied = true
			}

			// Attempt to merge entities
			mergeResult, mergeErr := g.mergeEntitiesImpl([]string{entityName}, newEntityName, nil, nil)
			if mergeErr != nil {
				logger.Errorf("Entity Edit: merge failed: %v", mergeErr)
				errStr := mergeErr.Error()
				operationSummary.Merged = false
				operationSummary.MergeStatus = "failed"
				operationSummary.MergeError = &errStr
				if nonNameUpdatesApplied {
					operationSummary.OperationStatus = "partial_success"
				} else {
					operationSummary.OperationStatus = "failure"
				}
				operationSummary.TargetEntity = &newEntityName
				operationSummary.FinalEntity = entityName

				// Return current entity info
				entityInfo, _ := g.GetEntityInfo(entityName, true)
				if entityInfo != nil {
					entityInfo.OperationSummary = operationSummary
					return entityInfo, nil
				}
				return &EntityInfo{
					EntityName:       entityName,
					OperationSummary: operationSummary,
				}, nil
			}

			// Merge succeeded
			operationSummary.Merged = true
			operationSummary.MergeStatus = "success"
			operationSummary.MergeError = nil
			operationSummary.OperationStatus = "success"
			operationSummary.TargetEntity = &newEntityName
			operationSummary.FinalEntity = newEntityName
			mergeResult.OperationSummary = operationSummary
			return mergeResult, nil
		}
	}

	// Normal edit flow (no merge involved)
	if err := g.editEntityImpl(entityName, updatedData); err != nil {
		return nil, err
	}

	entityInfo, err := g.GetEntityInfo(newEntityName, true)
	if err != nil {
		return nil, err
	}
	entityInfo.OperationSummary = operationSummary
	return entityInfo, nil
}

// editEntityImpl is the internal helper that performs entity edit operations.
func (g *GraphOperator) editEntityImpl(entityName string, updatedData map[string]string) error {
	newEntityName := entityName
	if name, ok := updatedData["entity_name"]; ok && name != "" {
		newEntityName = name
	}
	isRenaming := newEntityName != entityName

	// Check if entity exists
	nodeExists, err := g.Graph.HasNode(entityName)
	if err != nil {
		return fmt.Errorf("error checking entity: %w", err)
	}
	if !nodeExists {
		return fmt.Errorf("entity '%s' does not exist", entityName)
	}

	nodeData, err := g.Graph.GetNode(entityName)
	if err != nil {
		return fmt.Errorf("error getting entity data: %w", err)
	}

	if isRenaming {
		existingNode, err := g.Graph.HasNode(newEntityName)
		if err != nil {
			return fmt.Errorf("error checking new entity name: %w", err)
		}
		if existingNode {
			return fmt.Errorf("entity name '%s' already exists, cannot rename", newEntityName)
		}
	}

	// Create new node data with updates
	newNodeData := make(map[string]interface{})
	for k, v := range nodeData {
		newNodeData[k] = v
	}
	for k, v := range updatedData {
		if k != "entity_name" {
			newNodeData[k] = v
		}
	}
	newNodeData["entity_id"] = newEntityName

	if isRenaming {
		logger.Infof("Entity Edit: renaming `%s` to `%s`", entityName, newEntityName)

		// Create new node
		if err := g.Graph.UpsertNode(newEntityName, newNodeData); err != nil {
			return fmt.Errorf("error creating renamed node: %w", err)
		}

		// Handle relationship updates
		edges, err := g.Graph.GetNodeEdges(entityName)
		if err != nil {
			return fmt.Errorf("error getting edges: %w", err)
		}

		relationsToDelete := make([]string, 0)
		if edges != nil {
			for _, edge := range edges {
				src, tgt := edge[0], edge[1]
				edgeData, err := g.Graph.GetEdge(src, tgt)
				if err != nil {
					continue
				}

				relationsToDelete = append(relationsToDelete,
					ComputeMDHashID(src+tgt, "rel-"),
					ComputeMDHashID(tgt+src, "rel-"),
				)

				// Create new edge with updated entity name
				var newSrc, newTgt string
				if src == entityName {
					newSrc = newEntityName
					newTgt = tgt
				} else {
					newSrc = src
					newTgt = newEntityName
				}

				if err := g.Graph.UpsertEdge(newSrc, newTgt, edgeData); err != nil {
					logger.Warnf("Failed to update edge: %v", err)
				}

				// Update relation in VDB
				normalizedSrc, normalizedTgt := newSrc, newTgt
				if normalizedSrc > normalizedTgt {
					normalizedSrc, normalizedTgt = normalizedTgt, normalizedSrc
				}

				description, _ := edgeData["description"].(string)
				keywords, _ := edgeData["keywords"].(string)
				sourceID, _ := edgeData["source_id"].(string)
				weight := 1.0
				if w, ok := edgeData["weight"].(float64); ok {
					weight = w
				}

				content := fmt.Sprintf("%s\t%s\n%s\n%s", normalizedSrc, normalizedTgt, keywords, description)
				relationID := ComputeMDHashID(normalizedSrc+normalizedTgt, "rel-")

				relationData := map[string]interface{}{
					relationID: map[string]interface{}{
						"content":     content,
						"src_id":      normalizedSrc,
						"tgt_id":      normalizedTgt,
						"source_id":   sourceID,
						"description": description,
						"keywords":    keywords,
						"weight":      weight,
					},
				}
				if err := g.RelationshipsVDB.Upsert(relationData); err != nil {
					logger.Warnf("Failed to update relation in VDB: %v", err)
				}
			}
		}

		// Delete old node
		if err := g.Graph.DeleteNode(entityName); err != nil {
			return fmt.Errorf("error deleting old node: %w", err)
		}

		// Delete old entity from VDB
		oldEntityID := ComputeMDHashID(entityName, "ent-")
		if err := g.EntitiesVDB.Delete([]string{oldEntityID}); err != nil {
			logger.Warnf("Failed to delete old entity from VDB: %v", err)
		}

		// Delete old relations from VDB
		if len(relationsToDelete) > 0 {
			if err := g.RelationshipsVDB.Delete(relationsToDelete); err != nil {
				logger.Warnf("Failed to delete old relations from VDB: %v", err)
			}
		}
	} else {
		// Simple update without renaming
		if err := g.Graph.UpsertNode(entityName, newNodeData); err != nil {
			return fmt.Errorf("error updating node: %w", err)
		}
	}

	// Update entity in VDB
	description, _ := newNodeData["description"].(string)
	sourceID, _ := newNodeData["source_id"].(string)
	entityType, _ := newNodeData["entity_type"].(string)
	content := newEntityName + "\n" + description

	entityID := ComputeMDHashID(newEntityName, "ent-")
	entityData := map[string]interface{}{
		entityID: map[string]interface{}{
			"content":     content,
			"entity_name": newEntityName,
			"source_id":   sourceID,
			"description": description,
			"entity_type": entityType,
		},
	}
	if err := g.EntitiesVDB.Upsert(entityData); err != nil {
		return fmt.Errorf("error updating entity in VDB: %w", err)
	}

	// Update entity chunks storage
	if g.EntityChunksStorage != nil {
		newSourceID, _ := newNodeData["source_id"].(string)
		chunkIDs := splitSourceID(newSourceID)
		if len(chunkIDs) > 0 {
			chunkData := map[string]interface{}{
				newEntityName: map[string]interface{}{
					"chunk_ids": chunkIDs,
					"count":     len(chunkIDs),
				},
			}
			if err := g.EntityChunksStorage.Upsert(chunkData); err != nil {
				logger.Warnf("Failed to update entity chunks storage: %v", err)
			}
		}

		if isRenaming {
			if err := g.EntityChunksStorage.Delete([]string{entityName}); err != nil {
				logger.Warnf("Failed to delete old entity from chunks storage: %v", err)
			}
		}
	}

	if err := g.persistGraphUpdates(); err != nil {
		logger.Warnf("Failed to persist graph updates: %v", err)
	}

	logger.Infof("Entity Edit: `%s` successfully updated", newEntityName)
	return nil
}

// EditRelation edits relation information.
func (g *GraphOperator) EditRelation(sourceEntity, targetEntity string, updatedData map[string]interface{}) (*RelationInfo, error) {
	// Normalize entity order for undirected graph
	if sourceEntity > targetEntity {
		sourceEntity, targetEntity = targetEntity, sourceEntity
	}

	lock := g.getKeyedLock([]string{sourceEntity, targetEntity})
	lock.Lock()
	defer lock.Unlock()

	// Check if the relation exists
	edgeExists, err := g.Graph.HasEdge(sourceEntity, targetEntity)
	if err != nil {
		return nil, fmt.Errorf("error checking relation: %w", err)
	}
	if !edgeExists {
		return nil, fmt.Errorf("relation from '%s' to '%s' does not exist", sourceEntity, targetEntity)
	}

	edgeData, err := g.Graph.GetEdge(sourceEntity, targetEntity)
	if err != nil {
		return nil, fmt.Errorf("error getting edge data: %w", err)
	}

	// Delete old relation record from VDB
	relIDsToDelete := []string{
		ComputeMDHashID(sourceEntity+targetEntity, "rel-"),
		ComputeMDHashID(targetEntity+sourceEntity, "rel-"),
	}
	if err := g.RelationshipsVDB.Delete(relIDsToDelete); err != nil {
		logger.Warnf("Failed to delete old relation from VDB: %v", err)
	}

	// Update relation information in the graph
	newEdgeData := make(map[string]interface{})
	for k, v := range edgeData {
		newEdgeData[k] = v
	}
	for k, v := range updatedData {
		newEdgeData[k] = v
	}
	if err := g.Graph.UpsertEdge(sourceEntity, targetEntity, newEdgeData); err != nil {
		return nil, fmt.Errorf("error updating edge: %w", err)
	}

	// Update relation in VDB
	description, _ := newEdgeData["description"].(string)
	keywords, _ := newEdgeData["keywords"].(string)
	sourceID, _ := newEdgeData["source_id"].(string)
	weight := 1.0
	if w, ok := newEdgeData["weight"].(float64); ok {
		weight = w
	}

	content := fmt.Sprintf("%s\t%s\n%s\n%s", sourceEntity, targetEntity, keywords, description)
	relationID := ComputeMDHashID(sourceEntity+targetEntity, "rel-")

	relationData := map[string]interface{}{
		relationID: map[string]interface{}{
			"content":     content,
			"src_id":      sourceEntity,
			"tgt_id":      targetEntity,
			"source_id":   sourceID,
			"description": description,
			"keywords":    keywords,
			"weight":      weight,
		},
	}
	if err := g.RelationshipsVDB.Upsert(relationData); err != nil {
		return nil, fmt.Errorf("error updating relation in VDB: %w", err)
	}

	// Update relation chunks storage
	if g.RelationChunksStorage != nil {
		storageKey := MakeRelationChunkKey(sourceEntity, targetEntity)
		newSourceID, _ := newEdgeData["source_id"].(string)
		chunkIDs := splitSourceID(newSourceID)
		if len(chunkIDs) > 0 {
			chunkData := map[string]interface{}{
				storageKey: map[string]interface{}{
					"chunk_ids": chunkIDs,
					"count":     len(chunkIDs),
				},
			}
			if err := g.RelationChunksStorage.Upsert(chunkData); err != nil {
				logger.Warnf("Failed to update relation chunks storage: %v", err)
			}
		}
	}

	if err := g.persistGraphUpdates(); err != nil {
		logger.Warnf("Failed to persist graph updates: %v", err)
	}

	logger.Infof("Relation Edit: `%s`~`%s` successfully updated", sourceEntity, targetEntity)
	return g.GetRelationInfo(sourceEntity, targetEntity, true)
}

// CreateEntity creates a new entity.
func (g *GraphOperator) CreateEntity(entityName string, entityData map[string]interface{}) (*EntityInfo, error) {
	lock := g.getKeyedLock([]string{entityName})
	lock.Lock()
	defer lock.Unlock()

	// Check if entity already exists
	existingNode, err := g.Graph.HasNode(entityName)
	if err != nil {
		return nil, fmt.Errorf("error checking entity: %w", err)
	}
	if existingNode {
		return nil, fmt.Errorf("entity '%s' already exists", entityName)
	}

	// Prepare node data with defaults
	entityType := "UNKNOWN"
	if et, ok := entityData["entity_type"].(string); ok && et != "" {
		entityType = et
	}
	description := ""
	if desc, ok := entityData["description"].(string); ok {
		description = desc
	}
	sourceID := "manual_creation"
	if sid, ok := entityData["source_id"].(string); ok && sid != "" {
		sourceID = sid
	}
	filePath := "manual_creation"
	if fp, ok := entityData["file_path"].(string); ok && fp != "" {
		filePath = fp
	}

	nodeData := map[string]interface{}{
		"entity_id":   entityName,
		"entity_type": entityType,
		"description": description,
		"source_id":   sourceID,
		"file_path":   filePath,
		"created_at":  time.Now().Unix(),
	}

	// Add entity to knowledge graph
	if err := g.Graph.UpsertNode(entityName, nodeData); err != nil {
		return nil, fmt.Errorf("error creating node: %w", err)
	}

	// Prepare data for VDB
	content := entityName + "\n" + description
	entityID := ComputeMDHashID(entityName, "ent-")

	entityDataForVDB := map[string]interface{}{
		entityID: map[string]interface{}{
			"content":     content,
			"entity_name": entityName,
			"source_id":   sourceID,
			"description": description,
			"entity_type": entityType,
			"file_path":   filePath,
		},
	}

	// Update vector database
	if err := g.EntitiesVDB.Upsert(entityDataForVDB); err != nil {
		return nil, fmt.Errorf("error adding entity to VDB: %w", err)
	}

	// Update entity chunks storage
	if g.EntityChunksStorage != nil {
		chunkIDs := splitSourceID(sourceID)
		if len(chunkIDs) > 0 {
			chunkData := map[string]interface{}{
				entityName: map[string]interface{}{
					"chunk_ids": chunkIDs,
					"count":     len(chunkIDs),
				},
			}
			if err := g.EntityChunksStorage.Upsert(chunkData); err != nil {
				logger.Warnf("Failed to update entity chunks storage: %v", err)
			} else {
				logger.Infof("Entity Create: tracked %d chunks for `%s`", len(chunkIDs), entityName)
			}
		}
	}

	if err := g.persistGraphUpdates(); err != nil {
		logger.Warnf("Failed to persist graph updates: %v", err)
	}

	logger.Infof("Entity Create: '%s' successfully created", entityName)
	return g.GetEntityInfo(entityName, true)
}

// CreateRelation creates a new relation between entities.
func (g *GraphOperator) CreateRelation(sourceEntity, targetEntity string, relationData map[string]interface{}) (*RelationInfo, error) {
	lock := g.getKeyedLock([]string{sourceEntity, targetEntity})
	lock.Lock()
	defer lock.Unlock()

	// Check if both entities exist
	sourceExists, err := g.Graph.HasNode(sourceEntity)
	if err != nil {
		return nil, fmt.Errorf("error checking source entity: %w", err)
	}
	if !sourceExists {
		return nil, fmt.Errorf("source entity '%s' does not exist", sourceEntity)
	}

	targetExists, err := g.Graph.HasNode(targetEntity)
	if err != nil {
		return nil, fmt.Errorf("error checking target entity: %w", err)
	}
	if !targetExists {
		return nil, fmt.Errorf("target entity '%s' does not exist", targetEntity)
	}

	// Check if relation already exists
	existingEdge, err := g.Graph.HasEdge(sourceEntity, targetEntity)
	if err != nil {
		return nil, fmt.Errorf("error checking relation: %w", err)
	}
	if existingEdge {
		return nil, fmt.Errorf("relation from '%s' to '%s' already exists", sourceEntity, targetEntity)
	}

	// Prepare edge data with defaults
	description := ""
	if desc, ok := relationData["description"].(string); ok {
		description = desc
	}
	keywords := ""
	if kw, ok := relationData["keywords"].(string); ok {
		keywords = kw
	}
	sourceID := "manual_creation"
	if sid, ok := relationData["source_id"].(string); ok && sid != "" {
		sourceID = sid
	}
	weight := 1.0
	if w, ok := relationData["weight"].(float64); ok {
		weight = w
	}
	filePath := "manual_creation"
	if fp, ok := relationData["file_path"].(string); ok && fp != "" {
		filePath = fp
	}

	edgeData := map[string]interface{}{
		"description": description,
		"keywords":    keywords,
		"source_id":   sourceID,
		"weight":      weight,
		"file_path":   filePath,
		"created_at":  time.Now().Unix(),
	}

	// Add relation to knowledge graph
	if err := g.Graph.UpsertEdge(sourceEntity, targetEntity, edgeData); err != nil {
		return nil, fmt.Errorf("error creating edge: %w", err)
	}

	// Normalize entity order for VDB
	normalizedSrc, normalizedTgt := sourceEntity, targetEntity
	if normalizedSrc > normalizedTgt {
		normalizedSrc, normalizedTgt = normalizedTgt, normalizedSrc
	}

	// Prepare data for VDB
	content := fmt.Sprintf("%s\t%s\n%s\n%s", keywords, normalizedSrc, normalizedTgt, description)
	relationID := ComputeMDHashID(normalizedSrc+normalizedTgt, "rel-")

	relationDataForVDB := map[string]interface{}{
		relationID: map[string]interface{}{
			"content":     content,
			"src_id":      normalizedSrc,
			"tgt_id":      normalizedTgt,
			"source_id":   sourceID,
			"description": description,
			"keywords":    keywords,
			"weight":      weight,
			"file_path":   filePath,
		},
	}

	// Update vector database
	if err := g.RelationshipsVDB.Upsert(relationDataForVDB); err != nil {
		return nil, fmt.Errorf("error adding relation to VDB: %w", err)
	}

	// Update relation chunks storage
	if g.RelationChunksStorage != nil {
		storageKey := MakeRelationChunkKey(normalizedSrc, normalizedTgt)
		chunkIDs := splitSourceID(sourceID)
		if len(chunkIDs) > 0 {
			chunkData := map[string]interface{}{
				storageKey: map[string]interface{}{
					"chunk_ids": chunkIDs,
					"count":     len(chunkIDs),
				},
			}
			if err := g.RelationChunksStorage.Upsert(chunkData); err != nil {
				logger.Warnf("Failed to update relation chunks storage: %v", err)
			} else {
				logger.Infof("Relation Create: tracked %d chunks for `%s`~`%s`", len(chunkIDs), sourceEntity, targetEntity)
			}
		}
	}

	if err := g.persistGraphUpdates(); err != nil {
		logger.Warnf("Failed to persist graph updates: %v", err)
	}

	logger.Infof("Relation Create: `%s`~`%s` successfully created", sourceEntity, targetEntity)
	return g.GetRelationInfo(sourceEntity, targetEntity, true)
}

// MergeEntities merges multiple entities into one.
func (g *GraphOperator) MergeEntities(sourceEntities []string, targetEntity string, mergeStrategy map[string]MergeStrategy, targetEntityData map[string]interface{}) (*EntityInfo, error) {
	// Collect all entities and lock them all
	allEntities := make(map[string]bool)
	for _, e := range sourceEntities {
		allEntities[e] = true
	}
	allEntities[targetEntity] = true

	lockKeys := make([]string, 0, len(allEntities))
	for e := range allEntities {
		lockKeys = append(lockKeys, e)
	}

	lock := g.getKeyedLock(lockKeys)
	lock.Lock()
	defer lock.Unlock()

	return g.mergeEntitiesImpl(sourceEntities, targetEntity, mergeStrategy, targetEntityData)
}

// mergeEntitiesImpl is the internal helper for merging entities.
func (g *GraphOperator) mergeEntitiesImpl(sourceEntities []string, targetEntity string, mergeStrategy map[string]MergeStrategy, targetEntityData map[string]interface{}) (*EntityInfo, error) {
	// Default merge strategy
	defaultMergeStrategy := map[string]MergeStrategy{
		"description": MergeStrategyConcatenate,
		"entity_type": MergeStrategyKeepFirst,
		"source_id":   MergeStrategyJoinUnique,
		"file_path":   MergeStrategyJoinUnique,
	}

	effectiveMergeStrategy := defaultMergeStrategy
	if mergeStrategy != nil {
		logger.Warn("Entity Merge: merge_strategy parameter is deprecated and will be ignored in a future release.")
		for k, v := range mergeStrategy {
			effectiveMergeStrategy[k] = v
		}
	}

	if targetEntityData == nil {
		targetEntityData = make(map[string]interface{})
	}

	// Check if all source entities exist
	sourceEntitiesData := make([]map[string]interface{}, 0, len(sourceEntities))
	for _, entityName := range sourceEntities {
		nodeExists, err := g.Graph.HasNode(entityName)
		if err != nil {
			return nil, fmt.Errorf("error checking source entity '%s': %w", entityName, err)
		}
		if !nodeExists {
			return nil, fmt.Errorf("source entity '%s' does not exist", entityName)
		}
		nodeData, err := g.Graph.GetNode(entityName)
		if err != nil {
			return nil, fmt.Errorf("error getting source entity data '%s': %w", entityName, err)
		}
		sourceEntitiesData = append(sourceEntitiesData, nodeData)
	}

	// Check if target entity exists
	targetExists, err := g.Graph.HasNode(targetEntity)
	if err != nil {
		return nil, fmt.Errorf("error checking target entity: %w", err)
	}

	var existingTargetEntityData map[string]interface{}
	if targetExists {
		existingTargetEntityData, err = g.Graph.GetNode(targetEntity)
		if err != nil {
			return nil, fmt.Errorf("error getting target entity data: %w", err)
		}
	}

	// Merge entity data
	dataToMerge := sourceEntitiesData
	if targetExists && existingTargetEntityData != nil {
		dataToMerge = append(dataToMerge, existingTargetEntityData)
	}
	mergedEntityData := mergeAttributes(dataToMerge, effectiveMergeStrategy, false)

	// Apply explicitly provided target entity data
	for k, v := range targetEntityData {
		mergedEntityData[k] = v
	}

	// Get all relationships of source entities and target entity
	type relationInfo struct {
		src      string
		tgt      string
		edgeData map[string]interface{}
	}
	allRelations := make([]relationInfo, 0)
	entitiesToCollect := make([]string, len(sourceEntities))
	copy(entitiesToCollect, sourceEntities)

	if targetExists {
		found := false
		for _, e := range sourceEntities {
			if e == targetEntity {
				found = true
				break
			}
		}
		if !found {
			entitiesToCollect = append(entitiesToCollect, targetEntity)
		}
	}

	for _, entityName := range entitiesToCollect {
		edges, err := g.Graph.GetNodeEdges(entityName)
		if err != nil {
			continue
		}
		for _, edge := range edges {
			src, tgt := edge[0], edge[1]
			if src == entityName {
				edgeData, err := g.Graph.GetEdge(src, tgt)
				if err != nil {
					continue
				}
				allRelations = append(allRelations, relationInfo{src: src, tgt: tgt, edgeData: edgeData})
			}
		}
	}

	// Create or update the target entity
	mergedEntityData["entity_id"] = targetEntity
	if err := g.Graph.UpsertNode(targetEntity, mergedEntityData); err != nil {
		return nil, fmt.Errorf("error upserting target entity: %w", err)
	}
	if !targetExists {
		logger.Infof("Entity Merge: created target '%s'", targetEntity)
	} else {
		logger.Infof("Entity Merge: updated target '%s'", targetEntity)
	}

	// Process relations
	relationUpdates := make(map[string]struct {
		graphSrc string
		graphTgt string
		normSrc  string
		normTgt  string
		data     map[string]interface{}
	})
	relationsToDelete := make([]string, 0)

	for _, rel := range allRelations {
		relationsToDelete = append(relationsToDelete,
			ComputeMDHashID(rel.src+rel.tgt, "rel-"),
			ComputeMDHashID(rel.tgt+rel.src, "rel-"),
		)

		newSrc := rel.src
		newTgt := rel.tgt
		for _, srcEntity := range sourceEntities {
			if rel.src == srcEntity {
				newSrc = targetEntity
				break
			}
		}
		for _, srcEntity := range sourceEntities {
			if rel.tgt == srcEntity {
				newTgt = targetEntity
				break
			}
		}

		// Skip self-loops
		if newSrc == newTgt {
			logger.Infof("Entity Merge: skipping `%s`~`%s` to avoid self-loop", rel.src, rel.tgt)
			continue
		}

		// Normalize for consistent key
		normalizedSrc, normalizedTgt := newSrc, newTgt
		if normalizedSrc > normalizedTgt {
			normalizedSrc, normalizedTgt = normalizedTgt, normalizedSrc
		}
		relationKey := fmt.Sprintf("%s|%s", normalizedSrc, normalizedTgt)

		if existing, exists := relationUpdates[relationKey]; exists {
			// Merge relationship data
			mergedRelation := mergeAttributes(
				[]map[string]interface{}{existing.data, rel.edgeData},
				map[string]MergeStrategy{
					"description": MergeStrategyConcatenate,
					"keywords":    MergeStrategyJoinUniqueComma,
					"source_id":   MergeStrategyJoinUnique,
					"file_path":   MergeStrategyJoinUnique,
					"weight":      MergeStrategyMax,
				},
				true,
			)
			existing.data = mergedRelation
			relationUpdates[relationKey] = existing
		} else {
			edgeDataCopy := make(map[string]interface{})
			for k, v := range rel.edgeData {
				edgeDataCopy[k] = v
			}
			relationUpdates[relationKey] = struct {
				graphSrc string
				graphTgt string
				normSrc  string
				normTgt  string
				data     map[string]interface{}
			}{
				graphSrc: newSrc,
				graphTgt: newTgt,
				normSrc:  normalizedSrc,
				normTgt:  normalizedTgt,
				data:     edgeDataCopy,
			}
		}
	}

	// Apply relationship updates
	logger.Infof("Entity Merge: updating %d relations", len(relationUpdates))
	for _, relData := range relationUpdates {
		if err := g.Graph.UpsertEdge(relData.graphSrc, relData.graphTgt, relData.data); err != nil {
			logger.Warnf("Failed to upsert edge: %v", err)
		}
	}

	// Delete old relations from VDB
	if len(relationsToDelete) > 0 {
		if err := g.RelationshipsVDB.Delete(relationsToDelete); err != nil {
			logger.Warnf("Failed to delete old relations from VDB: %v", err)
		}
	}

	// Update relations in VDB
	for _, relData := range relationUpdates {
		description, _ := relData.data["description"].(string)
		keywords, _ := relData.data["keywords"].(string)
		sourceID, _ := relData.data["source_id"].(string)
		weight := 1.0
		if w, ok := relData.data["weight"].(float64); ok {
			weight = w
		}

		content := fmt.Sprintf("%s\t%s\n%s\n%s", keywords, relData.normSrc, relData.normTgt, description)
		relationID := ComputeMDHashID(relData.normSrc+relData.normTgt, "rel-")

		relationDataForVDB := map[string]interface{}{
			relationID: map[string]interface{}{
				"content":     content,
				"src_id":      relData.normSrc,
				"tgt_id":      relData.normTgt,
				"source_id":   sourceID,
				"description": description,
				"keywords":    keywords,
				"weight":      weight,
			},
		}
		if err := g.RelationshipsVDB.Upsert(relationDataForVDB); err != nil {
			logger.Warnf("Failed to upsert relation in VDB: %v", err)
		}
	}

	// Update entity in VDB
	description, _ := mergedEntityData["description"].(string)
	sourceID, _ := mergedEntityData["source_id"].(string)
	entityType, _ := mergedEntityData["entity_type"].(string)
	content := targetEntity + "\n" + description

	entityID := ComputeMDHashID(targetEntity, "ent-")
	entityDataForVDB := map[string]interface{}{
		entityID: map[string]interface{}{
			"content":     content,
			"entity_name": targetEntity,
			"source_id":   sourceID,
			"description": description,
			"entity_type": entityType,
		},
	}
	if err := g.EntitiesVDB.Upsert(entityDataForVDB); err != nil {
		return nil, fmt.Errorf("error updating entity in VDB: %w", err)
	}

	// Merge entity chunk tracking
	if g.EntityChunksStorage != nil {
		allChunkIDLists := make([][]string, 0)

		// Process all source entities and target entity
		entitiesToProcess := make([]string, 0)
		for _, entityName := range sourceEntities {
			if entityName != targetEntity {
				entitiesToProcess = append(entitiesToProcess, entityName)
			}
		}
		if targetExists {
			entitiesToProcess = append(entitiesToProcess, targetEntity)
		}

		for _, entityName := range entitiesToProcess {
			stored, err := g.EntityChunksStorage.GetByID(entityName)
			if err == nil && stored != nil {
				if chunkIDs, ok := stored["chunk_ids"].([]interface{}); ok {
					ids := make([]string, 0, len(chunkIDs))
					for _, id := range chunkIDs {
						if idStr, ok := id.(string); ok && idStr != "" {
							ids = append(ids, idStr)
						}
					}
					if len(ids) > 0 {
						allChunkIDLists = append(allChunkIDLists, ids)
					}
				}
			}
		}

		// Merge chunk IDs with deduplication
		mergedChunkIDs := make([]string, 0)
		seen := make(map[string]bool)
		for _, chunkIDList := range allChunkIDLists {
			for _, chunkID := range chunkIDList {
				if !seen[chunkID] {
					seen[chunkID] = true
					mergedChunkIDs = append(mergedChunkIDs, chunkID)
				}
			}
		}

		// Delete source entities' chunk tracking
		entityKeysToDelete := make([]string, 0)
		for _, e := range sourceEntities {
			if e != targetEntity {
				entityKeysToDelete = append(entityKeysToDelete, e)
			}
		}
		if len(entityKeysToDelete) > 0 {
			if err := g.EntityChunksStorage.Delete(entityKeysToDelete); err != nil {
				logger.Warnf("Failed to delete source entities from chunks storage: %v", err)
			}
		}

		// Update target entity's chunk tracking
		if len(mergedChunkIDs) > 0 {
			chunkData := map[string]interface{}{
				targetEntity: map[string]interface{}{
					"chunk_ids": mergedChunkIDs,
					"count":     len(mergedChunkIDs),
				},
			}
			if err := g.EntityChunksStorage.Upsert(chunkData); err != nil {
				logger.Warnf("Failed to update target entity chunks storage: %v", err)
			} else {
				logger.Infof("Entity Merge: find %d chunks related to '%s'", len(mergedChunkIDs), targetEntity)
			}
		}
	}

	// Delete source entities
	for _, entityName := range sourceEntities {
		if entityName == targetEntity {
			logger.Warnf("Entity Merge: source entity '%s' is same as target entity", entityName)
			continue
		}

		logger.Infof("Entity Merge: deleting '%s' from KG and vdb", entityName)

		// Delete from graph
		if err := g.Graph.DeleteNode(entityName); err != nil {
			logger.Warnf("Failed to delete source entity from graph: %v", err)
		}

		// Delete from VDB
		entityID := ComputeMDHashID(entityName, "ent-")
		if err := g.EntitiesVDB.Delete([]string{entityID}); err != nil {
			logger.Warnf("Failed to delete source entity from VDB: %v", err)
		}
	}

	if err := g.persistGraphUpdates(); err != nil {
		logger.Warnf("Failed to persist graph updates: %v", err)
	}

	logger.Infof("Entity Merge: successfully merged %d entities into '%s'", len(sourceEntities), targetEntity)
	return g.GetEntityInfo(targetEntity, true)
}

// GetEntityInfo retrieves detailed information about an entity.
func (g *GraphOperator) GetEntityInfo(entityName string, includeVectorData bool) (*EntityInfo, error) {
	nodeData, err := g.Graph.GetNode(entityName)
	if err != nil {
		return nil, fmt.Errorf("error getting entity: %w", err)
	}

	sourceID := ""
	if nodeData != nil {
		if sid, ok := nodeData["source_id"].(string); ok {
			sourceID = sid
		}
	}

	result := &EntityInfo{
		EntityName: entityName,
		SourceID:   sourceID,
		GraphData:  nodeData,
	}

	if includeVectorData && g.EntitiesVDB != nil {
		entityID := ComputeMDHashID(entityName, "ent-")
		vectorData, err := g.EntitiesVDB.GetByID(entityID)
		if err == nil {
			result.VectorData = vectorData
		}
	}

	return result, nil
}

// GetRelationInfo retrieves detailed information about a relation.
func (g *GraphOperator) GetRelationInfo(srcEntity, tgtEntity string, includeVectorData bool) (*RelationInfo, error) {
	edgeData, err := g.Graph.GetEdge(srcEntity, tgtEntity)
	if err != nil {
		return nil, fmt.Errorf("error getting relation: %w", err)
	}

	sourceID := ""
	if edgeData != nil {
		if sid, ok := edgeData["source_id"].(string); ok {
			sourceID = sid
		}
	}

	result := &RelationInfo{
		SrcEntity: srcEntity,
		TgtEntity: tgtEntity,
		SourceID:  sourceID,
		GraphData: edgeData,
	}

	if includeVectorData && g.RelationshipsVDB != nil {
		relID := ComputeMDHashID(srcEntity+tgtEntity, "rel-")
		vectorData, err := g.RelationshipsVDB.GetByID(relID)
		if err == nil {
			result.VectorData = vectorData
		}
	}

	return result, nil
}

// mergeAttributes merges attributes from multiple entities or relationships.
func mergeAttributes(dataList []map[string]interface{}, mergeStrategy map[string]MergeStrategy, filterNoneOnly bool) map[string]interface{} {
	mergedData := make(map[string]interface{})

	// Collect all possible keys
	allKeys := make(map[string]bool)
	for _, data := range dataList {
		for key := range data {
			allKeys[key] = true
		}
	}

	// Merge values for each key
	for key := range allKeys {
		// Get all values for this key
		values := make([]interface{}, 0)
		for _, data := range dataList {
			if val, exists := data[key]; exists {
				if filterNoneOnly {
					if val != nil {
						values = append(values, val)
					}
				} else {
					// Filter falsy values
					if isTruthy(val) {
						values = append(values, val)
					}
				}
			}
		}

		if len(values) == 0 {
			continue
		}

		// Merge values according to strategy
		strategy, ok := mergeStrategy[key]
		if !ok {
			strategy = MergeStrategyKeepFirst
		}

		switch strategy {
		case MergeStrategyConcatenate:
			// Convert all values to strings and join
			strValues := make([]string, 0, len(values))
			for _, v := range values {
				strValues = append(strValues, fmt.Sprintf("%v", v))
			}
			mergedData[key] = strings.Join(strValues, GraphFieldSep)

		case MergeStrategyKeepFirst:
			mergedData[key] = values[0]

		case MergeStrategyKeepLast:
			mergedData[key] = values[len(values)-1]

		case MergeStrategyJoinUnique:
			// Handle fields separated by GraphFieldSep
			uniqueItems := make(map[string]bool)
			orderedItems := make([]string, 0)
			for _, v := range values {
				items := strings.Split(fmt.Sprintf("%v", v), GraphFieldSep)
				for _, item := range items {
					if item != "" && !uniqueItems[item] {
						uniqueItems[item] = true
						orderedItems = append(orderedItems, item)
					}
				}
			}
			mergedData[key] = strings.Join(orderedItems, GraphFieldSep)

		case MergeStrategyJoinUniqueComma:
			// Handle fields separated by comma
			uniqueItems := make(map[string]bool)
			orderedItems := make([]string, 0)
			for _, v := range values {
				items := strings.Split(fmt.Sprintf("%v", v), ",")
				for _, item := range items {
					item = strings.TrimSpace(item)
					if item != "" && !uniqueItems[item] {
						uniqueItems[item] = true
						orderedItems = append(orderedItems, item)
					}
				}
			}
			sort.Strings(orderedItems)
			mergedData[key] = strings.Join(orderedItems, ",")

		case MergeStrategyMax:
			// For numeric fields
			maxVal := 0.0
			for i, v := range values {
				if fv, ok := toFloat64(v); ok {
					if i == 0 || fv > maxVal {
						maxVal = fv
					}
				}
			}
			mergedData[key] = maxVal

		default:
			mergedData[key] = values[0]
		}
	}

	return mergedData
}

// Helper functions

// splitSourceID splits a source ID string by the graph field separator.
func splitSourceID(sourceID string) []string {
	if sourceID == "" {
		return nil
	}
	parts := strings.Split(sourceID, GraphFieldSep)
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// isTruthy checks if a value is truthy (not empty, not zero, not nil).
func isTruthy(val interface{}) bool {
	if val == nil {
		return false
	}
	switch v := val.(type) {
	case string:
		return v != ""
	case int:
		return v != 0
	case int64:
		return v != 0
	case float64:
		return v != 0
	case bool:
		return v
	case []interface{}:
		return len(v) > 0
	case map[string]interface{}:
		return len(v) > 0
	default:
		return true
	}
}

// toFloat64 converts a value to float64 if possible.
func toFloat64(val interface{}) (float64, bool) {
	switch v := val.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	default:
		return 0, false
	}
}

// ComputeIncrementalChunkIDs computes incremental chunk IDs based on existing, old, and new chunk IDs.
func ComputeIncrementalChunkIDs(existingFullChunkIDs, oldChunkIDs, newChunkIDs []string) []string {
	// Create a set of old chunk IDs for quick lookup
	oldSet := make(map[string]bool)
	for _, id := range oldChunkIDs {
		oldSet[id] = true
	}

	// Start with existing chunk IDs
	result := make([]string, 0, len(existingFullChunkIDs))
	seen := make(map[string]bool)

	// Add existing chunk IDs that are not in old set
	for _, id := range existingFullChunkIDs {
		if !oldSet[id] && !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}

	// Add new chunk IDs
	for _, id := range newChunkIDs {
		if !seen[id] {
			result = append(result, id)
			seen[id] = true
		}
	}

	return result
}
