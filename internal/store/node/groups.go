package node

import (
	"context"
	"dash/internal/model"

	"gorm.io/gorm"
)

const defaultGroupName = "default"

type GroupItem struct {
	ID          int64  `json:"id"`
	Name        string `json:"name"`
	Remark      string `json:"remark"`
	ServerCount int64  `json:"server_count"`
}

type GroupNodes struct {
	ID      int64   `json:"id"`
	Name    string  `json:"name"`
	NodeIDs []int64 `json:"node_ids"`
}

func (s *Store) GroupsWithCounts(ctx context.Context) ([]GroupItem, error) {
	var groups []GroupItem
	err := s.db.WithContext(ctx).
		Table("groups AS g").
		Select("g.id", "g.name", "g.remark", "COUNT(s.id) AS server_count").
		Joins("LEFT JOIN server_groups sg ON sg.group_id = g.id").
		Joins("LEFT JOIN servers s ON s.id = sg.server_id AND s.is_deleted = ?", false).
		Where("g.is_deleted = ?", false).
		Group("g.id").
		Order("g.id ASC").
		Scan(&groups).
		Error
	return groups, err
}

func (s *Store) GroupNodes(ctx context.Context, guestVisibleOnly bool) ([]GroupNodes, error) {
	type row struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}

	var groups []row
	if err := s.db.WithContext(ctx).
		Table("groups").
		Select("id", "name").
		Where("is_deleted = ?", false).
		Order("id ASC").
		Scan(&groups).Error; err != nil {
		return nil, err
	}
	if len(groups) == 0 {
		return []GroupNodes{}, nil
	}

	groupIDs := make([]int64, 0, len(groups))
	for _, g := range groups {
		groupIDs = append(groupIDs, g.ID)
	}

	type relationRow struct {
		GroupID  int64 `json:"group_id"`
		ServerID int64 `json:"server_id"`
	}
	query := s.db.WithContext(ctx).
		Table("server_groups AS sg").
		Select("sg.group_id", "sg.server_id").
		Joins("JOIN servers s ON s.id = sg.server_id AND s.is_deleted = ?", false).
		Where("sg.group_id IN ?", groupIDs)
	if guestVisibleOnly {
		query = query.Where("s.is_guest_visible = ?", true)
	}

	var relations []relationRow
	if err := query.
		Order("sg.group_id ASC, sg.server_id ASC").
		Find(&relations).Error; err != nil {
		return nil, err
	}

	byGroup := make(map[int64][]int64, len(groups))
	for _, rel := range relations {
		byGroup[rel.GroupID] = append(byGroup[rel.GroupID], rel.ServerID)
	}

	out := make([]GroupNodes, 0, len(groups))
	for _, g := range groups {
		nodes := byGroup[g.ID]
		if nodes == nil {
			nodes = make([]int64, 0)
		}
		out = append(out, GroupNodes{ID: g.ID, Name: g.Name, NodeIDs: nodes})
	}
	return out, nil
}

func (s *Store) CreateGroup(ctx context.Context, name, remark string) (model.Group, error) {
	grp := model.Group{Name: name, Remark: remark}
	err := s.db.WithContext(ctx).Create(&grp).Error
	return grp, err
}

func (s *Store) UpdateGroup(ctx context.Context, id int64, updates map[string]any) error {
	res := s.db.WithContext(ctx).
		Model(&model.Group{}).
		Where("id = ? AND is_deleted = ?", id, false).
		Updates(updates)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (s *Store) DeleteGroup(ctx context.Context, id int64) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&model.Group{}).
			Where("id = ? AND is_deleted = ?", id, false).
			Update("is_deleted", true)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}
		return tx.Where("group_id = ?", id).Delete(&model.ServerGroup{}).Error
	})
}

func (s *Store) GroupLookup(ctx context.Context) (map[int64]string, error) {
	type row struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}

	var rows []row
	if err := s.db.WithContext(ctx).
		Model(&row{}).
		Table("groups").
		Select("id", "name").
		Where("is_deleted = ?", false).
		Order("id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	lookup := make(map[int64]string, len(rows))
	for _, row := range rows {
		lookup[row.ID] = row.Name
	}
	return lookup, nil
}

func ensureDefaultGroupID(ctx context.Context, tx *gorm.DB) (int64, error) {
	type row struct {
		ID int64
	}

	var active row
	if err := tx.WithContext(ctx).
		Table("groups").
		Select("id").
		Where("name = ? AND is_deleted = ?", defaultGroupName, false).
		Order("id ASC").
		Take(&active).Error; err == nil && active.ID > 0 {
		return active.ID, nil
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return 0, err
	}

	var deleted row
	if err := tx.WithContext(ctx).
		Table("groups").
		Select("id").
		Where("name = ?", defaultGroupName).
		Order("id ASC").
		Take(&deleted).Error; err == nil && deleted.ID > 0 {
		if err := tx.WithContext(ctx).
			Model(&model.Group{}).
			Where("id = ?", deleted.ID).
			Update("is_deleted", false).Error; err != nil {
			return 0, err
		}
		return deleted.ID, nil
	} else if err != nil && err != gorm.ErrRecordNotFound {
		return 0, err
	}

	grp := model.Group{Name: defaultGroupName, Remark: defaultGroupName}
	if err := tx.WithContext(ctx).Create(&grp).Error; err != nil {
		return 0, err
	}
	return grp.ID, nil
}

// EnsureDefaultGroup ensures the default group exists and is active.
func (s *Store) EnsureDefaultGroup(ctx context.Context) (int64, error) {
	var id int64
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var err error
		id, err = ensureDefaultGroupID(ctx, tx)
		return err
	})
	return id, err
}
