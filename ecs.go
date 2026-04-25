package main

type Entity int

type World struct {
	nextEntity Entity

	Positions    map[Entity]Position
	Velocities   map[Entity]Velocity
	Sprites      map[Entity]Sprite
	PlayerInputs map[Entity]PlayerInput
}

func NewWorld() *World {
	return &World{
		nextEntity:   1,
		Positions:    make(map[Entity]Position),
		Velocities:   make(map[Entity]Velocity),
		Sprites:      make(map[Entity]Sprite),
		PlayerInputs: make(map[Entity]PlayerInput),
	}
}

func (w *World) NewEntity() Entity {
	entity := w.nextEntity
	w.nextEntity++
	return entity
}

func (w *World) DeleteEntity(entity Entity) {
	delete(w.Positions, entity)
	delete(w.Velocities, entity)
	delete(w.Sprites, entity)
	delete(w.PlayerInputs, entity)
}
