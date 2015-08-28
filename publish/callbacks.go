package publish

import (
	"fmt"

	"github.com/jinzhu/gorm"
)

func isProductionModeAndNewScope(scope *gorm.Scope) (isProduction bool, clone *gorm.Scope) {
	if !IsDraftMode(scope.DB()) {
		if _, ok := scope.InstanceGet("publish:supported_model"); ok {
			table := OriginalTableName(scope.TableName())
			clone := scope.New(scope.Value)
			clone.Search.Table(table)
			return true, clone
		}
	}
	return false, nil
}

func setTableAndPublishStatus(ensureDraftMode bool) func(*gorm.Scope) {
	return func(scope *gorm.Scope) {
		if scope.Value == nil {
			return
		}

		if IsPublishableModel(scope.Value) {
			scope.InstanceSet("publish:supported_model", true)

			if ensureDraftMode {
				scope.Set("publish:force_draft_table", true)
				scope.Search.Table(DraftTableName(scope.TableName()))

				// Only set publish status when updating data from draft tables
				if IsDraftMode(scope.DB()) {
					if _, ok := scope.DB().Get(publishEvent); !ok {
						if attrs, ok := scope.InstanceGet("gorm:update_attrs"); ok {
							updateAttrs := attrs.(map[string]interface{})
							updateAttrs["publish_status"] = DIRTY
							scope.InstanceSet("gorm:update_attrs", updateAttrs)
						} else {
							scope.SetColumn("PublishStatus", DIRTY)
						}
					}
				}
			}
		}
	}
}

func syncCreateFromProductionToDraft(scope *gorm.Scope) {
	if !scope.HasError() {
		if ok, clone := isProductionModeAndNewScope(scope); ok {
			gorm.Create(clone)
		}
	}
}

func syncUpdateFromProductionToDraft(scope *gorm.Scope) {
	if !scope.HasError() {
		if ok, clone := isProductionModeAndNewScope(scope); ok {
			if updateAttrs, ok := scope.InstanceGet("gorm:update_attrs"); ok {
				table := OriginalTableName(scope.TableName())
				clone.Search = scope.Search
				clone.Search.Table(table)
				clone.InstanceSet("gorm:update_attrs", updateAttrs)
			}
			gorm.Update(clone)
		}
	}
}

func syncDeleteFromProductionToDraft(scope *gorm.Scope) {
	if !scope.HasError() {
		if ok, clone := isProductionModeAndNewScope(scope); ok {
			gorm.Delete(clone)
		}
	}
}

func deleteScope(scope *gorm.Scope) {
	if !scope.HasError() {
		_, supportedModel := scope.InstanceGet("publish:supported_model")
		if supportedModel && IsDraftMode(scope.DB()) {
			scope.Raw(
				fmt.Sprintf("UPDATE %v SET deleted_at=%v, publish_status=%v %v",
					scope.QuotedTableName(),
					scope.AddToVars(gorm.NowFunc()),
					scope.AddToVars(DIRTY),
					scope.CombinedConditionSql(),
				))
			scope.Exec()
		} else {
			gorm.Delete(scope)
		}
	}
}

func createPublishEvent(scope *gorm.Scope) {
	if event, ok := scope.Get(publishEvent); ok {
		if event, ok := event.(PublishEvent); ok {
			scope.Err(scope.NewDB().Save(&event).Error)
		}
	}
}
