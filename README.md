# g-engine-prototype

Protótipo de engine 2D em Go com OpenGL, GLFW, mapa em tiles e base multiplayer cliente/servidor.

## Como rodar

Servidor:

```bash
go run . -mode=server -addr=127.0.0.1:5000
```

Cliente:

```bash
go run . -mode=client -addr=127.0.0.1:5000
```

Se você não passar `-addr`, o projeto usa o valor de `defaultServerAddr` em `config.go`.

## Estrutura atual

Hoje o projeto está dividido em dois fluxos:

- `main.go`: ponto de entrada. Escolhe `client` ou `server`.
- `client.go`: abre janela, inicializa OpenGL, carrega shaders, mapa e texturas, conecta no servidor e desenha o mundo.
- `server.go`: aceita conexões, recebe input dos jogadores, simula posição e envia estado do mundo.
- `ecs.go`, `components.go`, `systems.go`: base ECS usada no cliente para representar entidades renderizáveis.
- `map.go`: carrega mapa exportado pelo Tiled e calcula tiles visíveis, UVs e transformações.
- `shader.go`: compila e linka shaders GLSL.
- `texture.go`: carrega PNG e envia a textura para a GPU.
- `network.go`: define o protocolo TCP em JSON entre cliente e servidor.

---

## Parte 1: Renderização e OpenGL

Esta parte explica como um tile do mapa ou um player sai do código Go e aparece na tela.

### Visão geral do pipeline

O fluxo de renderização do cliente hoje é este:

1. criar a janela com GLFW
2. criar o contexto OpenGL
3. inicializar a API OpenGL no processo Go
4. compilar os shaders
5. carregar o mapa e as texturas
6. criar um quad base na GPU
7. entrar no loop principal
8. em cada frame:
- ler input
- receber estado do servidor
- atualizar câmera
- limpar a tela
- desenhar tiles do mapa
- desenhar sprites dos jogadores
- trocar os buffers

Em outras palavras: o projeto não desenha "imagens soltas". Ele desenha sempre o mesmo quad, mas muda a matriz, a textura e a região da textura usada em cada draw call.

## 1. GLFW e contexto OpenGL

A primeira etapa do cliente acontece em `RunClient()`:

```go
window := initGLFW()
defer glfw.Terminate()

initOpenGL()
```

`initGLFW()` faz algumas coisas importantes:

- inicializa a GLFW
- define a versão desejada do OpenGL
- cria a janela
- torna o contexto OpenGL atual com `window.MakeContextCurrent()`
- liga `SwapInterval(1)` para sincronizar com o refresh vertical

Sem contexto OpenGL ativo, chamadas como `gl.CreateShader`, `gl.GenBuffers` ou `gl.TexImage2D` não funcionam.

### Por que `runtime.LockOSThread()` é obrigatório

No `init()` de `main.go` existe:

```go
runtime.LockOSThread()
```

Isso é essencial porque a maioria dos drivers OpenGL espera que o contexto seja criado e usado na mesma thread do sistema operacional. Em Go, goroutines podem mudar de thread. Esse lock impede essa troca para o fluxo principal.

Se você tirar isso, o projeto pode começar a falhar de forma estranha: shader não compila, textura não sobe, draw não acontece ou a janela abre preta.

## 2. Inicialização da API OpenGL

Depois da janela existir, o projeto chama:

```go
if err := gl.Init(); err != nil {
    log.Fatalln("erro ao inicializar OpenGL:", err)
}
```

Isso carrega os ponteiros das funções OpenGL para o processo atual. A biblioteca `go-gl` precisa disso para conseguir chamar a implementação do driver.

Aqui é importante entender uma regra:

- `GLFW` cria a janela e o contexto
- `gl.Init()` conecta seu programa Go às funções do driver

Uma coisa não substitui a outra.

## 3. Shaders: quem decide a posição e a cor final

O cliente cria o shader program com:

```go
program, err := NewShaderProgram("assets/shaders/basic.vert", "assets/shaders/basic.frag")
```

### O que `shader.go` faz

`shader.go` tem duas responsabilidades:

- ler o código GLSL dos arquivos `.vert` e `.frag`
- compilar e linkar esses shaders em um programa OpenGL

O fluxo é:

1. ler os arquivos com `os.ReadFile`
2. compilar vertex shader
3. compilar fragment shader
4. criar um programa
5. anexar os shaders
6. linkar
7. validar se o link funcionou
8. apagar os shaders intermediários

### Vertex shader

Arquivo: `assets/shaders/basic.vert`

Ele recebe:

- posição do vértice: `aPos`
- coordenada UV: `aTexCoord`
- uniforms `model`, `view`, `projection`
- uniforms `uvScale` e `uvOffset`

Função dele:

- transformar o vértice do espaço local para espaço de tela
- ajustar a UV para apontar para uma região específica da textura

Trecho central:

```glsl
gl_Position = projection * view * model * vec4(aPos, 1.0);
vTexCoord = aTexCoord * uvScale + uvOffset;
```

Isso é a alma da renderização atual do projeto.

### Fragment shader

Arquivo: `assets/shaders/basic.frag`

Ele é simples:

```glsl
FragColor = texture(texture0, vTexCoord);
```

Ou seja: para cada pixel do triângulo rasterizado, ele pega a cor correspondente da textura e escreve no framebuffer.

## 4. O mesh base: um único quad reutilizado para tudo

O projeto cria um único quad em `createQuadMesh()`.

Esse quad tem:

- 4 vértices
- 2 triângulos
- posição 3D
- UV por vértice

Os vértices são:

```go
vertices := []float32{
    -0.5, 0.5, 0.0, 0.0, 0.0,
    0.5, 0.5, 0.0, 1.0, 0.0,
    0.5, -0.5, 0.0, 1.0, 1.0,
    -0.5, -0.5, 0.0, 0.0, 1.0,
}
```

Isso quer dizer:

- o quad nasce centralizado na origem
- ele vai de `-0.5` até `0.5`
- no espaço local, ele tem tamanho 1x1
- a textura cobre o quad inteiro de UV `(0,0)` até `(1,1)`

Depois esse mesh é enviado para a GPU com:

- `VAO`: guarda a configuração dos atributos
- `VBO`: guarda os vértices
- `EBO`: guarda os índices

### Por que isso é bom

Porque você não precisa criar um mesh novo para cada tile ou player.

Você reutiliza sempre o mesmo quad e muda:

- a posição e escala via `model`
- a região da textura via `uvScale` e `uvOffset`
- a textura ativa com `gl.BindTexture`

Esse padrão é extremamente comum em engines 2D.

## 5. Matrizes: local, mundo, câmera e projeção

O projeto usa três matrizes clássicas:

- `model`: transforma um objeto do espaço local para o mundo
- `view`: representa a câmera
- `projection`: converte o mundo em coordenadas de tela

### Projection

No cliente:

```go
Projection: mgl32.Ortho(0, windowWidth, 0, windowHeight, -1, 1),
```

Isso cria uma projeção ortográfica 2D. Sem perspectiva. Em termos práticos:

- 1 unidade no mundo pode ser pensada como 1 pixel de tela
- `x` cresce para a direita
- `y` cresce para cima

Essa escolha combina muito com jogo 2D tileado.

### View

A câmera é atualizada a cada frame:

```go
renderer.View = mgl32.Translate3D(
    -cameraPos.X()+windowWidth/2,
    -cameraPos.Y()+windowHeight/2,
    0,
)
```

Isso desloca o mundo inteiro ao contrário da posição da câmera, de modo que o player local fique no centro da janela.

Regra importante:

- a câmera não move a janela
- a câmera move a visão do mundo
- em OpenGL, na prática, isso geralmente aparece como uma transformação inversa aplicada ao cenário

### Model

Cada tile e cada sprite monta sua própria model matrix.

Exemplo de sprite:

```go
model := mgl32.Translate3D(position.X(), position.Y(), position.Z()).
    Mul4(mgl32.Scale3D(sprite.Width, sprite.Height, 1))
```

O quad original era 1x1. Depois da escala, ele vira um sprite com largura e altura reais. Depois da translação, ele vai para sua posição no mundo.

## 6. Texturas e UV

O arquivo `texture.go` faz o upload de PNG para a GPU.

Fluxo:

1. abre o arquivo
2. decodifica a imagem
3. converte para RGBA
4. cria um objeto de textura OpenGL
5. configura wrap e filtering
6. envia os bytes com `gl.TexImage2D`

### Parâmetros escolhidos

Hoje a textura usa:

- `CLAMP_TO_EDGE`: evita repetir fora da borda
- `LINEAR`: suaviza a amostragem
- mipmap gerado com `gl.GenerateMipmap`

Para pixel art, futuramente talvez você queira experimentar `NEAREST` em vez de `LINEAR`, para evitar blur.

### UV normal e UV de atlas

Para o sprite do player, a UV é simples:

- `UVScale = (1,1)`
- `UVOffset = (0,0)`

Isso usa a textura inteira.

Para o mapa, a textura é um atlas. Então cada tile precisa dizer:

- qual fração da textura usar
- em que ponto essa fração começa

É isso que `TileUV()` em `map.go` calcula a partir do `gid` do Tiled.

## 7. Como o mapa é desenhado

O mapa é carregado por `LoadMap()` em `map.go`.

Ele faz:

1. ler o `.tmj`
2. fazer `json.Unmarshal`
3. pegar o primeiro tileset
4. carregar a imagem do atlas desse tileset

Depois, no render loop, o cliente chama:

```go
renderTileMap(tileMap, cameraPos, renderer)
```

### Visible range

Antes de desenhar, o código calcula quais colunas e linhas estão visíveis:

```go
startCol, endCol, startRow, endRow := tileMap.VisibleRange(cameraPos)
```

Isso evita desenhar os 2500 tiles o tempo todo sem necessidade. Ainda não é um sistema de otimização sofisticado, mas já é um começo correto.

### Tile por tile

Para cada tile visível:

1. pega o `gid` da layer
2. transforma esse `gid` em `uvScale` e `uvOffset`
3. calcula a `model matrix` do tile
4. faz `gl.DrawElements`

Cada tile desenhado é, no fundo, o mesmo quad com outra transformação e outra região do atlas.

## 8. Como os jogadores são desenhados

Os players também são desenhados com o mesmo quad.

Depois de receber o `world_state` do servidor, o cliente sincroniza o ECS:

- cria entidades para players novos
- atualiza `Position`
- garante que cada entidade tenha um `Sprite`
- remove entidades de players desconectados

Na renderização, `RenderSystem()` percorre `world.Sprites`:

1. pega a posição da entidade
2. monta a `model matrix`
3. configura `uvScale` e `uvOffset`
4. faz bind da textura do sprite
5. chama `gl.DrawElements`

Então o pipeline visual do mapa e dos players é quase o mesmo. A diferença principal é:

- mapa: UV vem do atlas de tiles
- player: UV usa a textura inteira do personagem

## 9. Loop principal de renderização

No `for !window.ShouldClose()` do cliente, a parte visual importante é:

```go
gl.ClearColor(0.08, 0.09, 0.12, 1.0)
gl.Clear(gl.COLOR_BUFFER_BIT)

gl.UseProgram(program)
renderTileMap(tileMap, cameraPos, renderer)
RenderSystem(world, renderer)

window.SwapBuffers()
```

### O que isso significa

- `gl.ClearColor`: define a cor usada para limpar a tela
- `gl.Clear`: limpa o framebuffer
- `gl.UseProgram`: ativa o shader program
- `renderTileMap`: desenha o cenário
- `RenderSystem`: desenha os players por cima
- `SwapBuffers`: mostra o frame pronto na janela

Esse `SwapBuffers` existe porque a janela trabalha com double buffering:

- um buffer está sendo exibido
- o outro está sendo desenhado
- no fim do frame, eles trocam

Isso evita flicker.

## 10. Ordem mental correta para entender este render

Se você quiser pensar do jeito mais útil possível, pense assim:

1. existe um quad base 1x1 na GPU
2. o shader sabe receber posição e UV
3. cada draw call muda uniforms
4. a `model` coloca o quad no lugar certo
5. a `view` desloca o mundo pela câmera
6. a `projection` adapta para a tela 2D
7. a UV escolhe que parte da textura será lida
8. o fragment shader pinta os pixels

Essa é a base inteira da renderização atual do projeto.

## 11. Limitações atuais desta parte

Hoje a renderização ainda é simples e direta:

- sem batching
- sem sprite animation
- sem depth ordering mais elaborado
- sem culling por chunks
- sem câmera desacoplada em um sistema próprio
- sem abstração de material ou render queue

Mas para um protótipo de engine, a fundação está certa. O projeto já trabalha com os elementos essenciais que depois podem virar uma arquitetura mais madura.

## 12. O que você precisa dominar desta Parte 1

Se você sair desta parte entendendo os pontos abaixo, já está muito bem:

- por que GLFW existe e por que OpenGL precisa do contexto
- por que `LockOSThread()` é necessário
- diferença entre shader, textura, buffer e VAO
- o que `model`, `view` e `projection` fazem
- por que um único quad pode virar mapa inteiro e sprites
- como UV transforma uma textura grande em vários tiles
- por que o render loop limpa, desenha e troca buffers em todo frame

---

## Próximas partes sugeridas

- Parte 2: mapa, atlas, coordenadas, câmera e por que o eixo Y do mapa exige atenção
- Parte 3: multiplayer, ticks, input, estado autoritativo e sincronização
- Parte 4: como transformar esse protótipo numa engine mais modular

---

## Parte 2: Mapa, atlas, coordenadas e câmera

Agora que a base da renderização ficou clara, a próxima peça importante é entender como o mundo 2D é organizado.

Nesta engine, o mapa não é só um arquivo visual. Ele define:

- o tamanho do mundo
- o tamanho lógico de cada tile
- quais imagens do atlas serão usadas
- como converter linha e coluna em posição no mundo
- quais tiles precisam ser desenhados com base na câmera

Se você entender bem esta parte, começa a ficar muito mais fácil mexer em colisão, spawn, chunking, câmera, layers e até editor futuramente.

## 1. O que é o arquivo `.tmj`

O arquivo `assets/maps/mapa1.tmj` é um mapa exportado pelo Tiled em JSON.

Ele traz informações como:

- `width`: quantidade de colunas
- `height`: quantidade de linhas
- `tilewidth`: largura de cada tile
- `tileheight`: altura de cada tile
- `layers`: camadas do mapa
- `tilesets`: tilesets usados

No seu mapa atual, os valores principais são:

- mapa com `50 x 50` tiles
- cada tile mede `16 x 16`
- o mundo total tem `800 x 800` unidades

Essa conta vem de:

```text
worldWidth  = width * tileWidth  = 50 * 16 = 800
worldHeight = height * tileHeight = 50 * 16 = 800
```

Então, apesar da janela ser `800 x 600`, o mundo do mapa é `800 x 800`.

Isso já explica um detalhe importante:

- a janela é a área visível
- o mapa é maior que a janela na vertical
- por isso a câmera precisa decidir qual trecho do mapa mostrar

## 2. Como `map.go` representa o mapa

O struct principal é:

```go
type TileMap struct {
    Width      int
    Height     int
    TileWidth  int
    TileHeight int
    Layers     []MapLayer
    Tilesets   []Tileset

    Texture *Texture
}
```

Mentalmente:

- `Width` e `Height`: tamanho do grid
- `TileWidth` e `TileHeight`: tamanho físico de cada célula
- `Layers`: dados das camadas
- `Tilesets`: metadados do atlas
- `Texture`: textura OpenGL carregada do tileset

Isso significa que o `TileMap` já mistura duas dimensões:

- dimensão de dados do mapa, vinda do JSON
- dimensão de renderização, ao guardar a textura do atlas já carregada

Essa mistura é normal em protótipo. Mais tarde você pode separar “asset de mapa” de “instância renderizável do mapa”.

## 3. Layers: o mapa é um conjunto de grades

Cada `MapLayer` contém:

- `Data`: um array de inteiros
- `Width`
- `Height`
- `Name`
- `Type`
- `Visible`

O campo mais importante é `Data`.

Ele é uma lista linear de inteiros, mas conceitualmente representa uma grade 2D. Cada inteiro é um `gid`.

### O que é `gid`

`gid` significa global tile id.

No Tiled, cada tile do tileset tem um identificador global. Quando uma célula do mapa usa um tile específico do atlas, ela guarda o `gid` correspondente.

Regras importantes:

- `gid == 0` significa célula vazia
- `gid > 0` aponta para algum tile real do tileset

No seu código, isso aparece em `TileUV()`:

```go
if gid == 0 || len(m.Tilesets) == 0 {
    return mgl32.Vec2{}, mgl32.Vec2{}, false
}
```

Ou seja: tile vazio nem tenta desenhar.

## 4. Como o tileset vira atlas

O mapa referencia um tileset cuja imagem é:

- `../textures/abaa.png`

Essa imagem é um atlas, isto é, uma textura grande contendo milhares de tiles menores.

Os metadados do tileset dizem, por exemplo:

- quantas colunas existem no atlas
- largura e altura da imagem total
- largura e altura de cada tile
- quantos tiles existem
- qual `firstgid` corresponde ao primeiro tile desse atlas

No seu caso, o tileset atual tem:

- `columns = 96`
- `imagewidth = 1536`
- `imageheight = 1024`
- `tilewidth = 16`
- `tileheight = 16`

Isso bate:

```text
1536 / 16 = 96 colunas
1024 / 16 = 64 linhas
96 * 64 = 6144 tiles
```

Perceba a elegância aqui: o mapa não guarda imagem por célula. Ele só guarda ids. A imagem real vem do atlas compartilhado.

## 5. Como `gid` vira UV

Esse é um dos pontos mais importantes do projeto.

Quando o código quer desenhar um tile, ele pega o `gid` e chama:

```go
uvScale, uvOffset, ok := tileMap.TileUV(gid)
```

### Passo a passo da conta

Dentro de `TileUV()`:

1. subtrai `firstgid` para descobrir o índice local do tile dentro do tileset
2. calcula a coluna com `%`
3. calcula a linha com `/`
4. calcula `uvScale`
5. calcula `uvOffset`

Exemplo mental:

Se o atlas tem `1536 x 1024` e cada tile tem `16 x 16`, então:

```text
uvScale.x = 16 / 1536
uvScale.y = 16 / 1024
```

Isso diz ao shader: “use só uma fração pequena da textura”.

Depois `uvOffset` diz onde essa fração começa.

### Tradução intuitiva

- `uvScale`: tamanho do recorte dentro da textura
- `uvOffset`: posição do recorte dentro da textura

O vertex shader então faz:

```glsl
vTexCoord = aTexCoord * uvScale + uvOffset;
```

Ou seja: a UV original do quad, que ia de `0` a `1`, é encolhida e deslocada para cair exatamente em cima do tile certo no atlas.

## 6. Como linha e coluna viram posição no mundo

O método `TileModel(row, col)` faz essa conversão:

```go
x := float32(col*m.TileWidth) + float32(m.TileWidth)/2
y := m.WorldHeight() - float32(row*m.TileHeight) - float32(m.TileHeight)/2
```

Esse trecho merece muita atenção.

### Eixo X

O `x` é tranquilo:

- coluna 0 começa na esquerda
- cada coluna anda `tileWidth`
- soma metade do tile para posicionar o centro do quad no centro da célula

Então o tile da coluna 0 fica centrado em `x = 8`, o da coluna 1 em `x = 24`, e assim por diante.

### Eixo Y

O `y` é o ponto delicado:

- no Tiled, as linhas crescem de cima para baixo
- no mundo usado pelo seu OpenGL ortográfico, o `y` cresce de baixo para cima

Por isso o código faz:

```go
y := m.WorldHeight() - float32(row*m.TileHeight) - float32(m.TileHeight)/2
```

Ele inverte o eixo vertical.

### Por que isso é necessário

Sem essa inversão:

- a linha 0 do mapa apareceria embaixo, e não em cima
- o mapa ficaria verticalmente espelhado

Esse é um dos pontos que mais confundem em engine 2D com editor externo.

Regra mental:

- Tiled pensa a grade a partir do canto superior esquerdo
- seu mundo renderizado pensa em coordenadas a partir do canto inferior esquerdo

O `TileModel()` é a ponte entre esses dois universos.

## 7. A janela não é o mundo

Uma diferença muito importante:

- `windowWidth/windowHeight` definem o tamanho da viewport visível
- `m.WorldWidth()/m.WorldHeight()` definem o tamanho total do mapa

No seu projeto:

- viewport: `800 x 600`
- mundo do mapa: `800 x 800`

Então a câmera mostra uma parte do mundo, não o mundo inteiro.

Essa distinção é essencial porque muita lógica futura depende disso:

- culling
- colisão por região
- spawn
- HUD separada do mundo
- zoom de câmera

## 8. Como a câmera decide o que é visível

Antes de desenhar o mapa, o código calcula:

```go
startCol, endCol, startRow, endRow := tileMap.VisibleRange(cameraPos)
```

A lógica de `VisibleRange()` transforma a posição da câmera em limites do retângulo visível no mundo.

Ela calcula:

- esquerda visível
- direita visível
- baixo visível
- cima visível

Depois converte essas bordas em índices de linha e coluna.

### Tradução da ideia

Se a câmera está centrada em `(cx, cy)`, então:

- a esquerda visível é `cx - windowWidth/2`
- a direita visível é `cx + windowWidth/2`
- a parte de baixo é `cy - windowHeight/2`
- a parte de cima é `cy + windowHeight/2`

Depois isso vira índices de tile com divisão por `TileWidth` e `TileHeight`.

### Por que existe margem de `-1` e `+1`

O código faz:

```go
startCol := floor(...) - 1
endCol := floor(...) + 1
```

Isso cria uma borda de segurança.

Motivo:

- evitar cortar tiles na borda por erro de arredondamento
- garantir que tiles parcialmente visíveis ainda sejam desenhados

É uma otimização conservadora. Bem aceitável para o estágio atual.

## 9. Clamp dos índices visíveis

Depois de calcular o range visível, o código corrige limites:

- se `startCol < 0`, vira `0`
- se `startRow < 0`, vira `0`
- se `endCol >= m.Width`, vira `m.Width - 1`
- se `endRow >= m.Height`, vira `m.Height - 1`

Isso evita acessar fora do array da layer.

Sem esse clamp, bastaria a câmera encostar numa borda para você correr risco de `index out of range`.

## 10. Como o índice linear da layer funciona

Dentro de `renderTileMap()`:

```go
index := row*layer.Width + col
gid := layer.Data[index]
```

Esse é o jeito padrão de mapear grade 2D em slice 1D.

Fórmula:

```text
index = row * quantidade_de_colunas + col
```

Então:

- para andar na horizontal, muda `col`
- para descer uma linha, soma `layer.Width`

Essa fórmula aparece o tempo todo em engine, mapa, grid, pathfinding e colisão.

Vale a pena deixá-la totalmente natural na cabeça.

## 11. Como a câmera segue o player

No cliente:

```go
cameraPos := mgl32.Vec3{tileMap.WorldWidth() / 2, tileMap.WorldHeight() / 2, 0}
if localEntity, ok := entityByPlayerID[client.PlayerID()]; ok {
    cameraPos = world.Positions[localEntity].Value
}
```

Ou seja:

- antes de conhecer o player local, a câmera começa no centro do mapa
- depois que o cliente recebe o `playerID` e o estado do mundo, ela passa a seguir a posição desse player

Isso é simples e funcional.

### O que ainda não existe aqui

Hoje a câmera:

- não tem suavização
- não tem limites
- não tem zoom
- não tem dead zone
- não tem sistema próprio

Então, quando o player encostar nas bordas do mundo, a câmera ainda pode mostrar espaço fora do mapa, dependendo da posição e da viewport.

Isso não quebra a renderização, mas é um ponto natural de melhoria.

## 12. O mapa visual e a layer de colisão

No `.tmj`, você já tem pelo menos:

- uma layer visual de blocos
- uma layer chamada `collision`

Hoje o render percorre todas as layers do tipo `tilelayer` visíveis:

```go
for _, layer := range tileMap.Layers {
    if !layer.Visible || layer.Type != "tilelayer" {
        continue
    }
    ...
}
```

Isso significa uma coisa importante:

- a sua layer `collision` também pode estar sendo desenhada, se tiver tiles e estiver visível

Arquiteturalmente, existe uma diferença forte entre:

- layers visuais
- layers lógicas

Mais cedo ou mais tarde você vai querer tratar isso explicitamente.

Exemplo de evolução futura:

- layer `ground`: renderiza
- layer `decor`: renderiza
- layer `collision`: não renderiza, só participa da física
- layer `spawn`: talvez nem seja tilelayer, talvez vire object layer

Hoje o projeto ainda não separa essas intenções.

## 13. Cuidado importante: o servidor não usa o mapa real

Aqui tem um detalhe arquitetural bem relevante.

O cliente usa o mapa real carregado de `mapa1.tmj`.

Mas o servidor calcula o mundo com:

```go
defaultMapTilesWide
defaultMapTilesHigh
defaultMapTileWidth
defaultMapTileHeight
```

Ou seja: o tamanho do mundo no servidor hoje vem de constantes, não do arquivo do mapa.

Se um dia você mudar o `.tmj` para outro tamanho e esquecer de mudar `config.go`, o cliente e o servidor podem passar a discordar sobre os limites do mundo.

Essa é uma das primeiras coisas que eu ajustaria numa próxima etapa de arquitetura.

## 14. Qual é o modelo mental mais útil para o mapa

Pensa no mapa em camadas de tradução:

1. o arquivo `.tmj` descreve um grid
2. cada célula do grid tem um `gid`
3. o `gid` aponta para um tile dentro do atlas
4. a linha e a coluna definem onde esse tile fica no mundo
5. a câmera define quais células do mundo valem a pena desenhar
6. o shader usa UV para recortar a imagem certa do atlas

Esse encadeamento é o coração de qualquer engine 2D baseada em tilemap.

## 15. O que você precisa dominar desta Parte 2

Se esta parte ficar sólida na sua cabeça, você ganha muita autonomia:

- diferença entre mapa em grid e mundo em coordenadas contínuas
- o que é `gid`
- como um atlas permite reutilizar uma única textura grande
- como `row` e `col` viram posição no mundo
- por que o eixo Y do Tiled precisa ser invertido
- como transformar viewport em tiles visíveis
- por que câmera, mundo e janela são coisas diferentes
- por que layers visuais e lógicas devem ser separadas com o tempo

## 16. Próximo passo ideal

A Parte 3 fecha o tripé do projeto atual, porque aí entra o outro lado da verdade do jogo:

- cliente envia input
- servidor simula
- cliente renderiza o estado sincronizado

Quando você entender isso junto com as Partes 1 e 2, o projeto inteiro atual começa a fazer sentido como sistema.

---

## Parte 3: Multiplayer, ticks, input e sincronização

Agora entramos na parte que mais muda o jeito de pensar o projeto.

Se na Parte 1 a pergunta era "como algo aparece na tela?" e na Parte 2 era "como o mundo está organizado?", aqui a pergunta é:

"quem decide o que é verdade no jogo?"

No seu projeto atual, a resposta é:

- o cliente captura input
- o servidor simula o mundo
- o cliente só renderiza o estado recebido

Isso é uma arquitetura clássica de servidor autoritativo, mesmo ainda estando em uma versão bem inicial.

## 1. A ideia central: o cliente não manda posição

Este é o conceito mais importante da Parte 3.

O cliente não manda:

- posição final
- velocidade final
- estado completo do player

Ele manda só a intenção de movimento:

```text
quero andar em X e Y
```

No seu código isso é:

```go
Type:  messageTypeInput,
MoveX: moveX,
MoveY: moveY,
```

Depois, o servidor usa esse input para atualizar a posição real do jogador.

### Por que isso é importante

Porque em multiplayer quase sempre você quer que a verdade do jogo viva no servidor.

Se o cliente pudesse mandar a própria posição:

- seria muito mais fácil trapacear
- clientes diferentes poderiam discordar do estado
- colisão e regras ficariam inconsistentes

Então seu projeto já está numa direção certa: input sobe, estado desce.

## 2. O protocolo de rede atual

O protocolo está em `network.go`.

Os tipos de mensagem são:

- `input`
- `welcome`
- `world_state`

### `input`

Enviado do cliente para o servidor.

Leva:

- `move_x`
- `move_y`

Esses valores representam um vetor de movimento normalizado ou zerado.

### `welcome`

Enviado do servidor para o cliente assim que a conexão é aceita.

Leva:

- `player_id`

Esse id é a identidade do cliente dentro da simulação.

### `world_state`

Enviado do servidor para todos os clientes em todo tick.

Leva:

- lista de players
- para cada player: `id`, `x`, `y`

Hoje o estado sincronizado é mínimo: só posição 2D e id.

## 3. Formato mental do protocolo

Se você quiser pensar no fluxo sem se perder em detalhes de implementação, pense assim:

Cliente para servidor:

```text
eu estou apertando essa direção
```

Servidor para cliente:

```text
este é o estado atual oficial dos jogadores
```

Essa simplicidade é boa agora porque reduz acoplamento e facilita depurar.

## 4. Como o cliente se conecta

O cliente chama:

```go
client, err := ConnectToServer(addr)
```

Dentro de `ConnectToServer()`:

1. abre uma conexão TCP com `net.Dial`
2. cria um `json.Encoder`
3. cria um canal `updates`
4. sobe uma goroutine `readLoop()`

Esse ponto é importante:

- o loop principal do jogo não fica bloqueado esperando rede
- a leitura da conexão acontece em paralelo

Isso é uma escolha muito natural em Go.

## 5. A goroutine de leitura do cliente

Em `readLoop()`:

1. cria um `json.Decoder`
2. fica em loop fazendo `Decode`
3. se chegar `welcome`, guarda o `playerID`
4. se chegar outra mensagem, joga no canal `updates`

### Por que usar canal

Porque isso desacopla:

- a thread lógica de rede
- o loop principal de renderização

A goroutine de rede só lê e empilha mensagens.
O loop principal decide quando consumir.

Isso deixa o sistema mais previsível do que atualizar o mundo do cliente de qualquer goroutine diretamente.

## 6. Onde o cliente envia input

No loop principal do cliente:

```go
moveX, moveY := collectNetworkInput(window)
if err := client.SendInput(moveX, moveY); err != nil {
    return fmt.Errorf("falha ao enviar input ao servidor: %w", err)
}
```

Esse trecho roda em todo frame.

Então hoje o comportamento é:

- a cada frame renderizado, o cliente manda o input atual

Isso é simples, mas tem uma implicação:

- a frequência de envio de input depende do FPS do cliente

Se o cliente rodar a 144 FPS, ele vai mandar muito mais mensagens do que um cliente a 30 FPS.

Não é necessariamente errado num protótipo, mas é uma característica importante do design atual.

## 7. Como o input é montado

`collectNetworkInput()` lê `W`, `A`, `S`, `D` e monta um vetor 2D.

Se houver movimento, esse vetor é normalizado:

```go
if movement.Len() > 0 {
    movement = movement.Normalize()
}
```

Isso evita o clássico problema da diagonal ser mais rápida.

Sem normalização:

- andar só para a direita teria magnitude 1
- andar diagonal teria magnitude maior que 1

Com normalização:

- toda direção tem o mesmo “comprimento” de movimento

Isso é um detalhe pequeno, mas muito correto.

## 8. Como o servidor aceita clientes

Em `RunServer()`:

1. abre `net.Listen("tcp", addr)`
2. cria a struct `Server`
3. sobe `acceptLoop()` em goroutine
4. entra em `tickLoop()`

Então o servidor tem dois ritmos paralelos:

- aceitar e ler conexões
- simular o mundo em ticks fixos

Esse desacoplamento é importante.

Porque aceitar mensagens e simular mundo não precisam rodar na mesma cadência.

## 9. A struct `Server`

Ela guarda:

- `listener`
- `nextID`
- `conns`
- `players`
- `mu`

### `conns`

Mapeia `playerID` para a conexão ativa e encoder correspondente.

### `players`

Mapeia `playerID` para o estado do jogador no servidor:

- `ID`
- `Position`
- `Input`

Aqui está a fonte da verdade da simulação.

O cliente pode até achar que está em outro lugar por latência visual, mas quem vale de fato é esse estado do servidor.

## 10. O papel do `welcome`

Quando um cliente conecta, o servidor:

1. gera um id novo
2. calcula um spawn
3. registra a conexão
4. registra o player
5. envia `welcome`

O `welcome` existe para o cliente descobrir:

- quem ele é dentro do estado compartilhado

Isso é necessário porque o `world_state` traz uma lista de todos os jogadores. O cliente precisa saber qual deles é o seu para a câmera seguir o player local.

## 11. Como o spawn inicial é calculado

Em `addClient()`:

```go
spawn := mgl32.Vec3{
    worldWidth()/2 + float32((playerID-1)%2)*float32(defaultMapTileWidth*3),
    worldHeight()/2 - float32((playerID-1)/2)*float32(defaultMapTileHeight*3),
    0,
}
```

O que essa fórmula faz:

- começa perto do centro do mundo
- espalha players em uma grade simples

Não é um sistema sofisticado de spawn, mas evita que todo mundo nasça exatamente no mesmo pixel.

Também revela uma coisa arquitetural:

- o servidor ainda não usa pontos de spawn vindos do mapa

Isso mais tarde pode migrar para object layers do Tiled ou algum sistema de cena.

## 12. Como o servidor recebe o input

Para cada cliente conectado, o servidor sobe:

```go
go s.readClientLoop(playerID, conn)
```

Essa goroutine:

1. lê mensagens JSON em loop
2. ignora mensagens que não sejam `input`
3. atualiza `player.Input`

Importante: ela não move o player na hora.

Ela só atualiza a intenção de movimento atual.

Esse detalhe é excelente, porque preserva a separação:

- leitura de rede coleta comandos
- tick da simulação aplica esses comandos

## 13. Tick fixo: onde o jogo realmente anda

O coração do servidor é `tickLoop()`:

```go
ticker := time.NewTicker(time.Duration(float64(time.Second) / serverTickRate))
deltaTime := float32(1.0 / serverTickRate)
for range ticker.C {
    s.stepWorld(deltaTime)
    s.broadcastWorldState()
}
```

Hoje `serverTickRate` é `20.0`.

Então:

- o servidor atualiza o mundo 20 vezes por segundo
- cada tick usa `deltaTime = 0.05`

### Por que isso é importante

No cliente, o FPS pode variar.
No servidor, a simulação tenta ser estável.

Isso é muito importante em multiplayer porque:

- a lógica do jogo não fica dependente da máquina do cliente
- o estado compartilhado evolui em um ritmo previsível

## 14. Como `stepWorld()` funciona

Dentro de `stepWorld()` o servidor:

1. percorre todos os players
2. pega o `Input` atual
3. normaliza se necessário
4. multiplica por `playerSpeed * deltaTime`
5. soma na posição
6. faz clamp nos limites do mundo

Em outras palavras:

```text
posição_nova = posição_antiga + direção * velocidade * deltaTime
```

Essa é a fórmula base de integração de movimento que você vai ver em muitos jogos 2D simples.

## 15. O que exatamente está sincronizado

Depois da simulação, o servidor monta:

```go
players = append(players, NetworkPlayerState{
    ID: player.ID,
    X:  player.Position.X(),
    Y:  player.Position.Y(),
})
```

Então o cliente recebe apenas:

- id
- x
- y

Isso quer dizer que o cliente hoje não recebe, por exemplo:

- direção do player
- animação
- estado de ataque
- hp
- layer/z-index
- frame da sprite

Por enquanto isso é ótimo, porque o sistema fica pequeno e inteligível.

## 16. Broadcast: o mesmo estado para todos

`broadcastWorldState()` monta uma única mensagem e a envia para cada conexão.

Conceitualmente:

- o servidor produz um snapshot do mundo
- envia esse snapshot para todos os clientes

Esse padrão é muito comum.

Mais tarde você pode sofisticar com:

- delta compression
- interesse por proximidade
- snapshots parciais
- entidades invisíveis por área

Mas o modelo base já é esse.

## 17. Como o cliente aplica o `world_state`

No loop do cliente, depois de enviar input, ele chama:

```go
drainWorldUpdates(client, world, entityByPlayerID, playerTexture)
```

Essa função consome tudo o que estiver disponível no canal `updates` naquele momento.

Se vier um `world_state`, ela chama:

```go
syncWorldState(world, entityByPlayerID, update.Players, playerTexture)
```

### O que `syncWorldState()` faz

1. cria um conjunto de players ativos
2. para cada player recebido:
- cria entidade se não existir
- garante o componente `Sprite`
- atualiza `Position`
3. depois remove entidades que não vieram mais no snapshot

Isso transforma o snapshot de rede em estado ECS local renderizável.

Esse ponto é muito bonito arquiteturalmente:

- rede entrega estado serializado
- cliente converte em entidades/componentes
- renderização opera em cima do ECS

## 18. O papel do `entityByPlayerID`

Esse map:

```go
map[int]Entity
```

faz a ponte entre dois mundos:

- `playerID` da rede
- `Entity` local do ECS

Sem isso, o cliente não saberia qual entidade ECS corresponde ao player `7`, por exemplo.

Então ele é uma camada de identidade entre protocolo e renderização.

## 19. O cliente não faz predição

Isto é muito importante notar.

Hoje o cliente:

- não move localmente o player antes da resposta do servidor
- não interpola snapshots
- não corrige erro suavemente

Ele simplesmente:

1. manda input
2. espera snapshot
3. atualiza posição para o valor recebido

### Consequência prática

Em rede local provavelmente vai parecer aceitável.

Com latência real, você tende a perceber:

- resposta menos imediata
- possível sensação de atraso
- movimento com cara de “teleporte curto” entre snapshots

Isso não é um defeito do código em si. É o comportamento esperado de um modelo sem client prediction nem interpolation.

## 20. O cliente também não usa `deltaTime` para mover players

No loop do cliente existe:

```go
currentFrame := glfw.GetTime()
_ = float32(currentFrame - lastFrame)
lastFrame = currentFrame
```

Mas esse `deltaTime` não está sendo usado para simular movimento local.

Isso faz sentido com a arquitetura atual, porque o cliente não é quem decide a física do player.

O movimento efetivo vem do servidor.

Então o `deltaTime` do cliente hoje serve mais como sobra de uma etapa anterior ou preparação para evoluções futuras.

## 21. O que os systems ECS estão fazendo nessa arquitetura

Hoje, dentro do fluxo multiplayer real:

- `RenderSystem()` é usado
- `InputSystem()` não é usado
- `MovementSystem()` não é usado

Isso revela algo importante sobre o estágio da arquitetura:

- o ECS nasceu com cara de simulação local
- o multiplayer atual usa o ECS mais como espelho renderizável do estado de rede

Não é problema. Só é bom você ter clareza disso para não achar que a simulação está passando pelos systems tradicionais do ECS.

Hoje ela não está.

## 22. Concorrência e proteção de estado

O servidor usa `sync.Mutex`.

Isso é necessário porque há acesso concorrente:

- `acceptLoop()` pode adicionar cliente
- `readClientLoop()` pode atualizar input
- `tickLoop()` pode simular e transmitir estado

Sem mutex, você teria risco de:

- corrida ao acessar maps
- estados inconsistentes
- panics

No cliente, o `playerID` também é protegido com `RWMutex`, porque a goroutine de rede escreve e o loop principal lê.

## 23. O que acontece quando um cliente desconecta

No servidor, se a leitura falhar:

- o loop encerra
- `removeClient(playerID)` é chamado

Isso remove:

- conexão
- player do mundo

Depois, no próximo broadcast, os outros clientes já não verão mais esse player.

No cliente, `syncWorldState()` remove entidades de players que sumiram do snapshot.

Isso fecha o ciclo corretamente.

## 24. Limitações atuais da arquitetura de rede

Hoje a rede é simples e funcional, mas ainda inicial:

- TCP em vez de UDP
- JSON em vez de binário
- input enviado por frame do cliente
- snapshot completo enviado a cada tick
- sem predição local
- sem interpolação
- sem rollback
- sem compressão
- sem timestamps explícitos
- sem separação entre mensagens confiáveis e não confiáveis

Nada disso impede o protótipo de funcionar. Só define o teto atual.

## 25. Por que TCP + JSON é aceitável aqui

Para protótipo, debugging e aprendizado, essa escolha é boa.

Vantagens:

- implementação simples
- fácil inspecionar mensagens
- menos atrito para validar a arquitetura

Desvantagens:

- overhead maior
- latência menos controlável
- head-of-line blocking do TCP
- custo de serialização maior que binário

Para aprender e montar a base, está ótimo.

## 26. O modelo mental mais útil desta Parte 3

Pensa no loop inteiro assim:

1. o cliente captura a intenção do jogador
2. envia essa intenção ao servidor
3. o servidor armazena o input mais recente
4. no tick fixo, o servidor simula todas as posições
5. o servidor gera um snapshot oficial
6. o cliente recebe o snapshot
7. converte snapshot em entidades locais
8. renderiza essas entidades

Esse é o circuito principal do jogo hoje.

## 27. O que você precisa dominar desta Parte 3

Se isso ficar claro, você já entende o multiplayer atual do projeto:

- diferença entre input e estado
- o que significa servidor autoritativo
- por que o servidor usa tick fixo
- por que o cliente pode ter FPS variável sem mandar na simulação
- como snapshots são convertidos em ECS local
- por que ainda não existe predição nem interpolação
- como a identidade do player é amarrada com `welcome` e `playerID`

## 28. O que eu faria depois desta Parte 3

Os próximos saltos naturais de arquitetura seriam:

- fazer o servidor carregar o mapa real, e não só constantes
- separar layers visuais de layers de colisão
- adicionar colisão baseada na layer `collision`
- mover spawn para dados do mapa
- adicionar interpolação visual no cliente
- talvez depois pensar em predição local

---

## Próximas partes sugeridas

- Parte 4: como transformar esse protótipo numa engine mais modular
