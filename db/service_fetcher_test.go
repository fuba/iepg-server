package db

import (
	"testing"

	"github.com/fuba/iepg-server/models"
)

func TestConvertToService_NormalBS(t *testing.T) {
	// NHK BS: Type=1, ChannelType=BS -> should become Type=2
	resp := &mirakurunServiceResponse{
		ID:        400101,
		ServiceID: 101,
		NetworkID: 4,
		Name:      "ＮＨＫ　ＢＳ",
		Type:      1,
		Channel: channelInfo{
			Type:    "BS",
			Channel: "BS15_0",
		},
	}
	service := convertToService(resp)

	if service.Type != 2 {
		t.Errorf("expected Type=2 for BS channel, got %d", service.Type)
	}
	if service.ChannelType != "BS" {
		t.Errorf("expected ChannelType=BS, got %s", service.ChannelType)
	}
	if service.ServiceID != 101 {
		t.Errorf("expected ServiceID=101, got %d", service.ServiceID)
	}
}

func TestConvertToService_Type192PreservesOriginalType(t *testing.T) {
	// スカパー！インフォ: Type=192, ChannelType=CS -> should keep Type=192
	resp := &mirakurunServiceResponse{
		ID:        600101,
		ServiceID: 101,
		NetworkID: 6,
		Name:      "スカパー！インフォ",
		Type:      192,
		Channel: channelInfo{
			Type:    "CS",
			Channel: "CS2",
		},
	}
	service := convertToService(resp)

	if service.Type != 192 {
		t.Errorf("expected Type=192 to be preserved, got %d", service.Type)
	}
}

func TestConvertToService_NormalGR(t *testing.T) {
	resp := &mirakurunServiceResponse{
		ID:        3273601024,
		ServiceID: 1024,
		NetworkID: 32736,
		Name:      "ＮＨＫ総合",
		Type:      1,
		Channel: channelInfo{
			Type:    "GR",
			Channel: "27",
		},
	}
	service := convertToService(resp)

	if service.Type != 1 {
		t.Errorf("expected Type=1 for GR channel, got %d", service.Type)
	}
}

func TestConvertToService_NormalCS(t *testing.T) {
	resp := &mirakurunServiceResponse{
		ID:        700296,
		ServiceID: 296,
		NetworkID: 7,
		Name:      "TBSチャンネル1",
		Type:      1,
		Channel: channelInfo{
			Type:    "CS",
			Channel: "CS4",
		},
	}
	service := convertToService(resp)

	if service.Type != 3 {
		t.Errorf("expected Type=3 for CS channel, got %d", service.Type)
	}
}

func TestServiceMapNotOverwrittenByType192(t *testing.T) {
	// Simulate the scenario where NHK BS and スカパー！インフォ share serviceId=101
	sm := models.NewServiceMap()

	// First: NHK BS (Type=1, ChannelType=BS)
	nhkBS := &mirakurunServiceResponse{
		ID:        400101,
		ServiceID: 101,
		NetworkID: 4,
		Name:      "ＮＨＫ　ＢＳ",
		Type:      1,
		Channel: channelInfo{
			Type:    "BS",
			Channel: "BS15_0",
		},
	}
	svc := convertToService(nhkBS)
	if svc.Type != 192 {
		sm.Update(svc)
	}

	// Second: スカパー！インフォ (Type=192, ChannelType=CS)
	skyper := &mirakurunServiceResponse{
		ID:        600101,
		ServiceID: 101,
		NetworkID: 6,
		Name:      "スカパー！インフォ",
		Type:      192,
		Channel: channelInfo{
			Type:    "CS",
			Channel: "CS2",
		},
	}
	svc2 := convertToService(skyper)
	if svc2.Type != 192 {
		sm.Update(svc2)
	}

	// Verify NHK BS is still in the map, not overwritten by スカパー！インフォ
	result, ok := sm.Get(101)
	if !ok {
		t.Fatal("expected service 101 to be in the map")
	}
	if result.Name != "ＮＨＫ　ＢＳ" {
		t.Errorf("expected Name=ＮＨＫ　ＢＳ, got %s", result.Name)
	}
	if result.Type != 2 {
		t.Errorf("expected Type=2 (BS), got %d", result.Type)
	}
}
