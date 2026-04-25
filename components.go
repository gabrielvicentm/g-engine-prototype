package main

import "github.com/go-gl/mathgl/mgl32"

type Position struct {
	Value mgl32.Vec3
}

type Velocity struct {
	Value mgl32.Vec3
}

type Sprite struct {
	Texture  *Texture
	Width    float32
	Height   float32
	UVScale  mgl32.Vec2
	UVOffset mgl32.Vec2
}

type PlayerInput struct {
	Speed float32
}
