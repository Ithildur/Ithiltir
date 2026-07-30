package node

import (
	"context"
	"reflect"
	"testing"

	"dash/internal/model"
)

func TestIntegrationGroupNodesGuestVisibleHidesGroupsWithoutVisibleNodes(t *testing.T) {
	st := newIntegrationStore(t)
	ctx := context.Background()

	visibleGroup := createGroupNodeGroup(t, st, "public")
	privateGroup := createGroupNodeGroup(t, st, "private")
	createGroupNodeGroup(t, st, "empty")

	visibleSrv := createGroupNodeServer(t, st, "public-node", true)
	privateSrv := createGroupNodeServer(t, st, "private-node", false)
	linkGroupNode(t, st, visibleGroup.ID, visibleSrv.ID)
	linkGroupNode(t, st, visibleGroup.ID, privateSrv.ID)
	linkGroupNode(t, st, privateGroup.ID, privateSrv.ID)

	got, err := st.GroupNodes(ctx, true)
	if err != nil {
		t.Fatalf("GroupNodes(guest) error = %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("GroupNodes(guest) returned %d groups, want 1: %#v", len(got), got)
	}
	if got[0].ID != visibleGroup.ID || got[0].Name != visibleGroup.Name {
		t.Fatalf("GroupNodes(guest)[0] = %#v, want group %q", got[0], visibleGroup.Name)
	}
	if !reflect.DeepEqual(got[0].NodeIDs, []int64{visibleSrv.ID}) {
		t.Fatalf("GroupNodes(guest)[0].NodeIDs = %#v, want [%d]", got[0].NodeIDs, visibleSrv.ID)
	}

	all, err := st.GroupNodes(ctx, false)
	if err != nil {
		t.Fatalf("GroupNodes(auth) error = %v", err)
	}
	if len(all) != 4 {
		t.Fatalf("GroupNodes(auth) returned %d groups, want 4: %#v", len(all), all)
	}
	if len(all[0].NodeIDs) != 0 {
		t.Fatalf("GroupNodes(auth)[0].NodeIDs = %#v, want empty default group", all[0].NodeIDs)
	}
	if !reflect.DeepEqual(all[1].NodeIDs, []int64{visibleSrv.ID, privateSrv.ID}) {
		t.Fatalf("GroupNodes(auth)[1].NodeIDs = %#v, want [%d %d]", all[1].NodeIDs, visibleSrv.ID, privateSrv.ID)
	}
	if !reflect.DeepEqual(all[2].NodeIDs, []int64{privateSrv.ID}) {
		t.Fatalf("GroupNodes(auth)[2].NodeIDs = %#v, want [%d]", all[2].NodeIDs, privateSrv.ID)
	}
	if len(all[3].NodeIDs) != 0 {
		t.Fatalf("GroupNodes(auth)[3].NodeIDs = %#v, want empty", all[3].NodeIDs)
	}
}

func createGroupNodeGroup(t *testing.T, st *Store, name string) model.Group {
	t.Helper()

	group := model.Group{Name: name}
	if err := st.db.Create(&group).Error; err != nil {
		t.Fatalf("Create(group) error = %v", err)
	}
	return group
}

func createGroupNodeServer(t *testing.T, st *Store, name string, guestVisible bool) model.Server {
	t.Helper()

	srv := model.Server{
		Name:           name,
		Hostname:       name,
		Secret:         name + "-secret",
		IsGuestVisible: guestVisible,
	}
	if err := st.db.Create(&srv).Error; err != nil {
		t.Fatalf("Create(server) error = %v", err)
	}
	return srv
}

func linkGroupNode(t *testing.T, st *Store, groupID, serverID int64) {
	t.Helper()

	link := model.ServerGroup{GroupID: groupID, ServerID: serverID}
	if err := st.db.Create(&link).Error; err != nil {
		t.Fatalf("Create(server_group) error = %v", err)
	}
}
