package main

import (
	"context"
	"github.com/go-redis/redis/v8"
	"log"
)

var ctx = context.TODO()

func wasAlreadyPosted(id string) bool {
	val, err := r.Get(ctx, id).Result()
	if err != nil {
		if err == redis.Nil {
			log.Println("TT does not exists.", val)
			err = r.Set(ctx, id, "", 0).Err() // Adding TT to redis
		}
		return false
	}
	return true // Skip
}
