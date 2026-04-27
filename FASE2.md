# Fase 2

Concordo com esse plano quase por inteiro. A ordem está boa e bem mais alinhada com engenharia prática do que a versão inicial.

Só fiz uma leitura mais operacional: mantive o foco em estabilizar simulação, remover regras hardcoded e melhorar a percepção do multiplayer antes de expandir o mundo.

## Objetivo

Transformar o protótipo atual em um sandbox multiplayer consistente, com:

- colisão funcional
- spawn configurado pelo mapa
- separação clara entre visual e lógica
- sincronização visual melhor
- base pronta para entidades e animação

## Estado atual assumido

O projeto já tem:

- janela OpenGL
- renderer de sprite
- câmera ortográfica
- tilemap do Tiled
- ECS básico
- multiplayer TCP funcional
- sincronização básica entre players
- `VisibleRange()` para culling parcial

## Plano de ação

### 1. Colisão com tiles

Implementar o primeiro bloco crítico:

- componente `Collider`
- leitura da layer de colisão do mapa
- detecção AABB
- resolução separada por eixo `X` e `Y`

Direção recomendada:

```go
newX := pos.X + vel.X * dt
newY := pos.Y + vel.Y * dt

if !Collides(newX, pos.Y) {
    pos.X = newX
}

if !Collides(pos.X, newY) {
    pos.Y = newY
}
```

Resultado:

- player não atravessa parede
- movimento ganha regra física básica

### 2. Spawn points vindos do mapa

Remover spawn hardcoded do servidor.

No Tiled:

- criar `Object Layer: Spawns`
- definir objetos como `player_spawn`, `npc_spawn`, `item_spawn`

No código:

- carregar spawn points do mapa
- fazer o servidor escolher um ponto válido ao conectar

Resultado:

- spawn configurável
- menos acoplamento com constantes

### 3. Separação formal de layers

Separar claramente:

- layers visuais
- layer de colisão

Estrutura desejada:

- `GroundLayer`
- `DecorationLayer`
- `CollisionLayer`

Resultado:

- menos bug estrutural
- mapa mais fácil de evoluir

### 4. Interpolação no cliente

Melhorar a suavidade do multiplayer.

Estrutura:

- guardar posição anterior
- guardar posição atual
- renderizar com `lerp`

Ideia:

```go
renderPos = Lerp(prevPos, currPos, alpha)
```

Resultado:

- menos jitter
- players remotos mais suaves

### 5. Direção do player

Adicionar orientação baseada no movimento:

- `UP`
- `DOWN`
- `LEFT`
- `RIGHT`

Resultado:

- base correta para animação
- estado visual mais coerente

### 6. Sistema de animação

Depois que movimento e direção estiverem sólidos:

- sprite sheet
- animações `Idle` e `Walk`
- variantes por direção

Estrutura sugerida:

```go
type Animation struct {
    Frames []int
    Speed  float32
    Timer  float32
    Index  int
}
```

Resultado:

- player com feedback visual real

### 7. Entidades reais

Expandir o mundo com:

- NPC
- item
- objeto

Exemplos de componentes:

- NPC: `Position`, `Sprite`, `Collider`, `AI`
- Item: `Position`, `Sprite`, `Pickup`
- Objeto: `Position`, `Collider`

Resultado:

- mundo deixa de ser vazio
- ECS começa a mostrar valor de verdade

### 8. Refinamento de performance

Só depois da base jogável funcionar:

- batching de render
- chunking do tilemap
- culling mais refinado

Observação:

`VisibleRange()` já resolve parte do problema hoje, então esta etapa é refinamento, não fundação.

## Resumo da ordem

1. colisão
2. spawn via mapa
3. separação de layers
4. interpolação
5. direção
6. animação
7. entidades reais
8. performance

## Avaliação curta

Concordo com o roadmap. Ele está bom porque respeita as dependências reais deste projeto: primeiro solidifica mundo e simulação, depois melhora apresentação e expansão.
