package main

import (
	"log"

	"github.com/go-gl/gl/v4.1-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
	"github.com/go-gl/mathgl/mgl32"
)

type RenderContext struct {
	VAO               uint32
	IndexCount        int32
	ModelUniform      int32
	ViewUniform       int32
	ProjectionUniform int32
	UVScaleUniform    int32
	UVOffsetUniform   int32
	View              mgl32.Mat4
	Projection        mgl32.Mat4
}

func InputSystem(world *World, window *glfw.Window) {
	for entity, input := range world.PlayerInputs {
		velocity, ok := world.Velocities[entity]
		if !ok {
			continue
		}

		var movement mgl32.Vec3
		if window.GetKey(glfw.KeyW) == glfw.Press {
			movement[1] += 1
		}
		if window.GetKey(glfw.KeyS) == glfw.Press {
			movement[1] -= 1
		}
		if window.GetKey(glfw.KeyA) == glfw.Press {
			movement[0] -= 1
		}
		if window.GetKey(glfw.KeyD) == glfw.Press {
			movement[0] += 1
		}

		if movement.Len() > 0 {
			movement = movement.Normalize()
		}

		velocity.Value = movement.Mul(input.Speed)
		world.Velocities[entity] = velocity
	}
}

func MovementSystem(world *World, deltaTime float32) {
	for entity, position := range world.Positions {
		velocity, ok := world.Velocities[entity]
		if !ok {
			continue
		}

		position.Value = position.Value.Add(velocity.Value.Mul(deltaTime))
		world.Positions[entity] = position
	}
}

func RenderSystem(world *World, renderer *RenderContext) {
	gl.UniformMatrix4fv(renderer.ViewUniform, 1, false, &renderer.View[0])
	gl.UniformMatrix4fv(renderer.ProjectionUniform, 1, false, &renderer.Projection[0])
	gl.BindVertexArray(renderer.VAO)
	gl.ActiveTexture(gl.TEXTURE0)

	for entity, sprite := range world.Sprites {
		position, ok := world.Positions[entity]
		if !ok || sprite.Texture == nil {
			continue
		}

		model := mgl32.Translate3D(position.Value.X(), position.Value.Y(), position.Value.Z()).
			Mul4(mgl32.Scale3D(sprite.Width, sprite.Height, 1))

		gl.UniformMatrix4fv(renderer.ModelUniform, 1, false, &model[0])
		gl.Uniform2f(renderer.UVScaleUniform, sprite.UVScale.X(), sprite.UVScale.Y())
		gl.Uniform2f(renderer.UVOffsetUniform, sprite.UVOffset.X(), sprite.UVOffset.Y())
		gl.BindTexture(gl.TEXTURE_2D, sprite.Texture.ID)
		gl.DrawElements(gl.TRIANGLES, renderer.IndexCount, gl.UNSIGNED_INT, gl.PtrOffset(0))
	}
}

func LogPlayerMovement(world *World, deltaTime float32) {
	for entity := range world.PlayerInputs {
		position, hasPosition := world.Positions[entity]
		velocity, hasVelocity := world.Velocities[entity]
		if !hasPosition || !hasVelocity || velocity.Value.Len() == 0 {
			continue
		}

		log.Printf(
			"entity=%d velocidade=(%.2f, %.2f) posicao=(%.2f, %.2f) deltaTime=%.4f",
			entity,
			velocity.Value.X(),
			velocity.Value.Y(),
			position.Value.X(),
			position.Value.Y(),
			deltaTime,
		)
	}
}
