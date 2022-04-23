package mongodb

import (
	"GIG/app/databases/mongodb"
	"GIG/app/repositories/constants"
	"log"
	"time"

	"github.com/lsflk/gig-sdk/models"
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
)

type EntityRepository struct {
}

func (e EntityRepository) newEntityCollection() *mongodb.Collection {
	c := mongodb.NewCollectionSession("entities")
	textIndex := mgo.Index{
		Key: []string{"$text:title", "$text:search_text"},
		Weights: map[string]int{
			"title":       1,
			"search_text": 1,
		},
		Name: "textIndex",
	}
	titleIndex := mgo.Index{
		Key:    []string{"title"},
		Name:   "titleIndex",
		Unique: true,
	}
	c.Collection.EnsureIndex(textIndex)
	c.Collection.EnsureIndex(titleIndex)
	return c
}

/*
AddEntity insert a new Entity into database and returns
last inserted entity on success.
*/
func (e EntityRepository) AddEntity(entity models.Entity) (models.Entity, error) {
	c := e.newEntityCollection()
	defer c.Close()
	return entity, c.Collection.Insert(entity)
}

func (e EntityRepository) GetEntityByPreviousTitle(title string, date time.Time) (models.Entity, error) {
	var (
		entity models.Entity
		err    error
	)

	query := bson.M{
		"attributes.titles.values.value_string": title,
		"attributes.titles.values.date":         bson.M{"$lt": date.Add(time.Duration(1) * time.Second)},
	}

	c := e.newEntityCollection()
	defer c.Close()

	err = c.Collection.Find(query).Sort("-attributes.titles.values.date").One(&entity)
	return entity, err
}

/**
GetEntities Get all Entities where a given title is linked from
list of models.Entity on success
*/
func (e EntityRepository) GetRelatedEntities(entity models.Entity, limit int, offset int) ([]models.Entity, error) {
	var (
		entities []models.Entity
		err      error
	)

	query := bson.M{}
	c := e.newEntityCollection()
	defer c.Close()

	entityTitle := entity.GetTitle()
	if entityTitle != "" {
		query = bson.M{"links.title": bson.M{"$in": append(entity.GetLinkTitles(), entity.GetTitle())}}
	}
	log.Println(query)
	err = c.Collection.Find(query).Sort(constants.UpdatedAtDecending).Skip(offset).Limit(limit).All(&entities)

	for _, item := range entities {
		log.Println(item.GetTitle())
	}
	return entities, err
}

/**
GetEntities Get all Entities from database and returns
list of models.Entity on success
*/
func (e EntityRepository) GetEntities(search string, categories []string, limit int, offset int) ([]models.Entity, error) {
	var (
		entities    []models.Entity
		err         error
		resultQuery *mgo.Query
	)

	query := bson.M{}
	c := e.newEntityCollection()
	defer c.Close()

	if search != "" {
		query = bson.M{
			"$text": bson.M{"$search": search},
			//"attributes": bson.M{"$exists": true, "$not": bson.M{"$size": 0}},
		}
	}

	if categories != nil && len(categories) != 0 {
		query["categories"] = bson.M{"$all": categories}
	}

	// sort by search score for text indexed search, otherwise sort by latest first in category
	if search == "" {
		resultQuery = c.Collection.Find(query).Sort("-source_date")
	} else {
		resultQuery = c.Collection.Find(query).Select(bson.M{
			"score": bson.M{"$meta": "textScore"}}).Sort("$textScore:score")
	}

	err = resultQuery.Skip(offset).Limit(limit).All(&entities)

	return entities, err
}

/**
GetEntity Get a Entity from database and returns
a models. Entity on success
*/
func (e EntityRepository) GetEntity(id bson.ObjectId) (models.Entity, error) {
	var (
		entity models.Entity
		err    error
	)

	c := e.newEntityCollection()
	defer c.Close()

	err = c.Collection.Find(bson.M{"_id": id}).One(&entity)
	return entity, err
}

/**
GetEntity Get a Entity from database and returns
a models.Entity on success
*/
func (e EntityRepository) GetEntityBy(attribute string, value string) (models.Entity, error) {
	var (
		entity models.Entity
		err    error
	)

	c := e.newEntityCollection()
	defer c.Close()
	err = c.Collection.Find(bson.M{attribute: value}).Sort(constants.UpdatedAtDecending).One(&entity)
	return entity, err
}

/**
UpdateEntity update a Entity into database and returns
last nil on success.
*/
func (e EntityRepository) UpdateEntity(entity models.Entity) error {
	c := e.newEntityCollection()
	defer c.Close()

	err := c.Collection.Update(bson.M{
		"_id": entity.GetId(),
	}, bson.M{
		"$set": entity,
	})
	return err
}

/**
DeleteEntity Delete Entity from database and returns
last nil on success.
*/
func (e EntityRepository) DeleteEntity(entity models.Entity) error {
	c := e.newEntityCollection()
	defer c.Close()

	err := c.Collection.Remove(bson.M{"_id": entity.GetId()})
	return err
}

/**
GetStats Get entity states from the DB
*/
func (e EntityRepository) GetStats() (models.EntityStats, error) {
	var (
		entityStats models.EntityStats
		err         error
	)

	c := e.newEntityCollection()
	defer c.Close()

	// Get total number of entities
	entityStats.EntityCount, err = c.Collection.Find(nil).Count()
	var linkCount []map[string]interface{}

	//Get category wise count
	categoryCountPipeline := []bson.M{
		{constants.UnwindAttribute: constants.CategoryAttribute},
		{constants.GroupAttribute: bson.M{
			"_id":            constants.CategoryAttribute,
			"category_count": bson.M{"$sum": 1}}},
		{constants.SortAttribute: bson.M{"category_count": -1}},
	}
	err = c.Collection.Pipe(categoryCountPipeline).All(&entityStats.CategoryWiseCount)

	//Get category group wise count
	categoryGroupCountPipeline := []bson.M{
		{constants.UnwindAttribute: constants.CategoryAttribute},
		{constants.SortAttribute: bson.M{"categories": 1}},
		{constants.GroupAttribute: bson.M{"_id": "$_id", "sortedCategories": bson.M{"$push": constants.CategoryAttribute}}},
		{
			constants.GroupAttribute: bson.M{
				"_id":            "$sortedCategories",
				"category_count": bson.M{"$sum": 1}}},
		{constants.SortAttribute: bson.M{"category_count": -1}},
	}
	err = c.Collection.Pipe(categoryGroupCountPipeline).All(&entityStats.CategoryGroupWiseCount)

	// Get total number of relations
	linkSumPipeline := []bson.M{{
		constants.GroupAttribute: bson.M{
			"_id":      "$link_sum",
			"link_sum": bson.M{"$sum": bson.M{"$size": "$links"}}}}}

	err = c.Collection.Pipe(linkSumPipeline).All(&linkCount)
	entityStats.RelationCount, _ = linkCount[0]["link_sum"].(int)

	return entityStats, err
}
