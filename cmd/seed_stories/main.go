// Copyright (C) 2025-2026 Jose R F Junior <web2ajax@gmail.com>
// SPDX-License-Identifier: AGPL-3.0-or-later

// seed_stories populates the "stories" collection in NietzscheDB with
// therapeutic narratives, fables, koans, and wisdom tales.
// EVA uses these for narrative therapy via the WisdomService.
// Run: go run cmd/seed_stories/main.go
package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"eva/internal/brainstem/config"
	"eva/internal/brainstem/database"
	nietzscheInfra "eva/internal/brainstem/infrastructure/nietzsche"
	"eva/internal/hippocampus/knowledge"

	"github.com/joho/godotenv"
)

type Story struct {
	Title     string
	Content   string
	Archetype string // "trickster", "hero", "wise_elder", "helper", "shadow"
	Moral     string
	Tags      string // comma-separated
	Source    string // "aesop", "zen", "rumi", "nasrudin", "therapeutic", "african"
}

func main() {
	godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Config load failed: %v", err)
	}

	nzClient, err := nietzscheInfra.NewClient(cfg.NietzscheGRPCAddr)
	if err != nil {
		log.Fatalf("NietzscheDB connect failed: %v", err)
	}
	defer nzClient.Close()

	db := database.NewNietzscheDB(nzClient.SDK())
	vectorAdapter := nietzscheInfra.NewVectorAdapter(nzClient)

	var embedSvc *knowledge.EmbeddingService
	embedSvc, err = knowledge.NewEmbeddingService(cfg, vectorAdapter)
	if err != nil {
		log.Printf("WARN: Embedding service unavailable: %v — stories will be stored without vectors", err)
	}

	ctx := context.Background()
	stories := getAllStories()
	now := time.Now().Format(time.RFC3339)

	inserted := 0
	skipped := 0
	embedOK := 0

	for _, s := range stories {
		// Check if story already exists
		rows, _ := db.QueryByLabel(ctx, "story",
			" AND n.title = $title",
			map[string]interface{}{"title": s.Title}, 1)

		if len(rows) > 0 {
			skipped++
			continue
		}

		content := map[string]interface{}{
			"title":      s.Title,
			"content":    s.Content,
			"archetype":  s.Archetype,
			"moral":      s.Moral,
			"tags":       s.Tags,
			"source":     s.Source,
			"created_at": now,
		}

		id, err := db.Insert(ctx, "story", content)
		if err != nil {
			log.Printf("ERROR inserting %s: %v", s.Title, err)
			continue
		}
		inserted++

		// Generate embedding and store in stories vector collection
		if embedSvc != nil {
			embText := s.Title + ". " + s.Content + " Moral: " + s.Moral
			embedding, err := embedSvc.GenerateEmbedding(ctx, embText)
			if err != nil {
				log.Printf("WARN: embedding failed for %s: %v", s.Title, err)
			} else {
				payload := map[string]interface{}{
					"id":        id,
					"title":     s.Title,
					"content":   s.Content,
					"archetype": s.Archetype,
					"moral":     s.Moral,
					"tags":      s.Tags,
					"source":    s.Source,
				}
				if err := vectorAdapter.Upsert(ctx, "stories", fmt.Sprintf("%d", id), embedding, payload); err != nil {
					log.Printf("WARN: vector upsert failed for %s: %v", s.Title, err)
				} else {
					embedOK++
				}
			}
			// Rate limit: Gemini API has limits
			time.Sleep(500 * time.Millisecond)
		}

		log.Printf("OK [%s] %s (%s)", s.Source, s.Title, s.Archetype)
	}

	fmt.Printf("\n=== Seed Stories Complete ===\n")
	fmt.Printf("Inserted: %d | Skipped: %d | Embeddings: %d | Total: %d\n",
		inserted, skipped, embedOK, len(stories))
}

func getAllStories() []Story {
	return []Story{
		// === FÁBULAS TERAPÊUTICAS ===
		{
			Title:     "O Velho e a Árvore",
			Content:   "Um velho plantou uma oliveira. Os vizinhos riram: 'Nunca verás os frutos!' O velho sorriu: 'Também não vi quem plantou a árvore que me dá sombra hoje.'",
			Archetype: "wise_elder",
			Moral:     "Plantamos para quem vem depois de nós. A generosidade transcende o tempo.",
			Tags:      "generosidade,legado,paciência,tempo",
			Source:    "therapeutic",
		},
		{
			Title:     "A Pedra no Caminho",
			Content:   "Toda manhã o ancião tropeçava na mesma pedra. Um dia, em vez de praguejar, sentou-se nela. Descobriu que dali via o nascer do sol como de nenhum outro lugar.",
			Archetype: "wise_elder",
			Moral:     "Os obstáculos podem tornar-se pontos de observação privilegiados.",
			Tags:      "resiliência,perspectiva,obstáculo,transformação",
			Source:    "therapeutic",
		},
		{
			Title:     "O Cântaro Rachado",
			Content:   "Um cântaro rachado envergonhava-se por perder água no caminho. O jardineiro mostrou-lhe as flores que cresciam apenas do seu lado do trilho. 'As tuas imperfeições alimentaram beleza que ninguém previu.'",
			Archetype: "helper",
			Moral:     "As nossas imperfeições podem criar algo belo e inesperado.",
			Tags:      "imperfeição,autoestima,beleza,vulnerabilidade",
			Source:    "therapeutic",
		},
		{
			Title:     "O Tecelão Cego",
			Content:   "Um tecelão cego tecia tapetes que todos admiravam. 'Como fazes?' perguntaram. 'Sinto cada fio. Quando não vemos, tocamos com mais cuidado.' Os seus tapetes eram os mais perfeitos da aldeia.",
			Archetype: "hero",
			Moral:     "A perda de uma capacidade pode refinar as outras. A limitação gera atenção.",
			Tags:      "limitação,superação,atenção,cuidado",
			Source:    "therapeutic",
		},

		// === KOANS ZEN ===
		{
			Title:     "A Chávena de Chá",
			Content:   "Um professor visitou um mestre Zen. O mestre serviu chá e continuou a verter mesmo com a chávena cheia. 'Está a transbordar!' disse o professor. 'Como esta chávena, estás cheio das tuas opiniões. Como posso mostrar-te o Zen se não esvaziares primeiro?'",
			Archetype: "wise_elder",
			Moral:     "Para aprender, é preciso primeiro esvaziar-se do que julgamos saber.",
			Tags:      "humildade,aprendizagem,mente aberta,zen",
			Source:    "zen",
		},
		{
			Title:     "O Som de Uma Mão",
			Content:   "O mestre perguntou ao discípulo: 'Qual é o som de uma mão a bater palmas?' O discípulo tentou muitas respostas. Depois de anos, parou de procurar a resposta e encontrou a paz.",
			Archetype: "wise_elder",
			Moral:     "Nem tudo precisa de resposta. Às vezes a pergunta é o caminho.",
			Tags:      "meditação,aceitação,paz,paradoxo",
			Source:    "zen",
		},

		// === HISTÓRIAS DE NASRUDIN ===
		{
			Title:     "As Chaves de Nasrudin",
			Content:   "Nasrudin procurava as chaves debaixo do candeeiro da rua. 'Onde as perdeste?' 'Dentro de casa.' 'Então porque procuras aqui?' 'Porque aqui há mais luz!'",
			Archetype: "trickster",
			Moral:     "Procuramos soluções onde é confortável, não onde está o problema.",
			Tags:      "autoconhecimento,conforto,verdade,humor",
			Source:    "nasrudin",
		},
		{
			Title:     "O Burro de Nasrudin",
			Content:   "Nasrudin montou o burro ao contrário. 'Estás virado para trás!' disseram. 'Não sou eu que estou virado. É o burro que anda para o lado errado.'",
			Archetype: "trickster",
			Moral:     "A perspectiva depende de onde nos posicionamos. Quem define o que é 'certo'?",
			Tags:      "perspectiva,humor,relatividade,sabedoria",
			Source:    "nasrudin",
		},

		// === POEMAS DE RUMI (resumidos) ===
		{
			Title:     "A Casa de Hóspedes",
			Content:   "O ser humano é como uma casa de hóspedes. Cada manhã chega um novo visitante: uma alegria, uma tristeza, uma mesquinhez. Recebe-os a todos e trata-os bem, pois cada um pode ser um guia enviado de longe.",
			Archetype: "wise_elder",
			Moral:     "Todas as emoções são visitantes temporárias. Acolhê-las é sabedoria.",
			Tags:      "emoções,aceitação,hospitalidade,mindfulness",
			Source:    "rumi",
		},
		{
			Title:     "A Ferida é o Lugar",
			Content:   "A ferida é o lugar por onde a luz entra. Não fujas da tua dor. É ali, precisamente ali, que encontrarás a transformação.",
			Archetype: "helper",
			Moral:     "A dor e a vulnerabilidade são portas para o crescimento.",
			Tags:      "dor,transformação,luz,vulnerabilidade,cura",
			Source:    "rumi",
		},

		// === CONTOS AFRICANOS ===
		{
			Title:     "O Baobá e o Vento",
			Content:   "O baobá não luta contra o vento. As suas raízes são tão profundas que nenhuma tempestade o derruba. Os jovens perguntaram: 'Como resistes?' O baobá respondeu: 'Cresci para baixo antes de crescer para cima.'",
			Archetype: "wise_elder",
			Moral:     "A verdadeira força vem das raízes profundas, não da altura visível.",
			Tags:      "raízes,força,resiliência,paciência,África",
			Source:    "african",
		},
		{
			Title:     "A Tartaruga e a Chuva",
			Content:   "Quando chove, a tartaruga não corre. Leva consigo o abrigo. Os outros animais correm desesperados, mas a tartaruga continua no seu passo. 'Não tens medo?' 'A minha casa está sempre comigo.'",
			Archetype: "wise_elder",
			Moral:     "A segurança verdadeira não está no exterior, mas dentro de nós.",
			Tags:      "segurança,calma,autoconfiança,sabedoria,África",
			Source:    "african",
		},
		{
			Title:     "O Ancião e o Rio",
			Content:   "Um ancião sentava-se junto ao rio todos os dias. 'O que observas?' perguntaram. 'Observo que a água nunca é a mesma, mas o rio é sempre o mesmo.' Sorriu. 'Assim somos nós: mudamos todos os dias, mas continuamos a ser nós mesmos.'",
			Archetype: "wise_elder",
			Moral:     "A identidade persiste através da mudança. Mudar não é perder-se.",
			Tags:      "identidade,mudança,continuidade,sabedoria,envelhecimento",
			Source:    "african",
		},

		// === FÁBULAS DE ESOPO ===
		{
			Title:     "A Lebre e a Tartaruga",
			Content:   "A lebre gabava-se da sua velocidade e desafiou a tartaruga. Confiante, adormeceu a meio do caminho. A tartaruga, passo a passo, sem parar, cruzou a meta primeiro.",
			Archetype: "hero",
			Moral:     "A constância e a perseverança vencem a arrogância e a pressa.",
			Tags:      "perseverança,humildade,constância,paciência",
			Source:    "aesop",
		},
		{
			Title:     "O Leão e o Rato",
			Content:   "Um leão poupou a vida a um rato. Dias depois, o leão caiu numa rede. O pequeno rato roeu as cordas e libertou-o. 'Disseste que eu era demasiado pequeno para te ajudar.'",
			Archetype: "helper",
			Moral:     "Ninguém é demasiado pequeno para fazer a diferença. A bondade volta sempre.",
			Tags:      "bondade,reciprocidade,humildade,ajuda",
			Source:    "aesop",
		},

		// === HISTÓRIAS TERAPÊUTICAS PARA IDOSOS ===
		{
			Title:     "O Álbum de Fotografias",
			Content:   "A avó mostrava o álbum ao neto. Em cada foto, recordava uma história. O neto percebeu: a avó não estava a ver fotos — estava a revisitar versões de si mesma. Cada foto era uma ponte entre quem foi e quem é.",
			Archetype: "wise_elder",
			Moral:     "As memórias não são o passado — são a ponte entre quem fomos e quem somos.",
			Tags:      "memória,identidade,reminiscência,família,envelhecimento",
			Source:    "therapeutic",
		},
		{
			Title:     "As Mãos do Avô",
			Content:   "O neto olhava para as mãos enrugadas do avô. 'São feias?' perguntou o avô. 'Não. Cada ruga conta uma história. Esta é da vez que me salvaste. Esta é do pão que amassavas.' O avô chorou de alegria.",
			Archetype: "wise_elder",
			Moral:     "As marcas do tempo são um mapa da vida vivida. Cada ruga é uma história de amor.",
			Tags:      "envelhecimento,aceitação,amor,legado,família",
			Source:    "therapeutic",
		},
		{
			Title:     "A Semente Guardada",
			Content:   "Uma mulher de 80 anos plantou uma semente. 'Para quê?' perguntaram. 'Para provar que a esperança não tem idade.' A árvore cresceu e deu sombra a três gerações.",
			Archetype: "hero",
			Moral:     "A esperança é o acto mais corajoso que existe, em qualquer idade.",
			Tags:      "esperança,coragem,legado,propósito,envelhecimento",
			Source:    "therapeutic",
		},
		{
			Title:     "O Silêncio Partilhado",
			Content:   "Dois velhos amigos sentavam-se no banco do jardim. Não falavam. Depois de uma hora, um disse: 'Boa conversa.' O outro concordou. Tinham partilhado o mais precioso: presença sem exigência.",
			Archetype: "helper",
			Moral:     "A verdadeira companhia não precisa de palavras. Estar presente é o maior presente.",
			Tags:      "presença,amizade,silêncio,companhia,solidão",
			Source:    "therapeutic",
		},
	}
}
