package main

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
)

func main(){
  app := fiber.New()

  app.Post("/test", func(c *fiber.Ctx) error {
    fmt.Println(string(c.Body()))
    return nil
  })
  app.Listen(":8000")
}
