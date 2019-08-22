package entity_handlers

import (
	"GIG/app/models"
	"GIG/app/repositories"
	"GIG/scripts/entity_handlers"
)

func (t *TestEntityHandlers) TestThatAddEntitiesAsLinksWorks() {
	linkEntity := models.Entity{Title: "Sri Lanka"}

	entity := models.Entity{Title: "test entity"}
	entity, _ = entity_handlers.AddEntitiesAsLinks(entity, append([]models.Entity{}, linkEntity))
	entity = repositories.EagerLoad(entity)
	t.AssertEqual(entity.LoadedLinks[0].Title, "Sri Lanka")

}