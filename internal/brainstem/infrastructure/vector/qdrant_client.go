package vector

import (
	"context"
	"fmt"
	"log"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// QdrantClient cliente para Qdrant vector database
type QdrantClient struct {
	client qdrant.CollectionsClient
	points qdrant.PointsClient
	conn   *grpc.ClientConn
}

// NewQdrantClient cria um novo cliente Qdrant
func NewQdrantClient(host string, port int) (*QdrantClient, error) {
	addr := fmt.Sprintf("%s:%d", host, port)

	conn, err := grpc.Dial(
		addr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Qdrant: %w", err)
	}

	return &QdrantClient{
		client: qdrant.NewCollectionsClient(conn),
		points: qdrant.NewPointsClient(conn),
		conn:   conn,
	}, nil
}

// Close fecha a conexão
func (q *QdrantClient) Close() error {
	return q.conn.Close()
}

// CreateCollection cria uma coleção
func (q *QdrantClient) CreateCollection(
	ctx context.Context,
	name string,
	vectorSize uint64,
) error {
	_, err := q.client.Create(ctx, &qdrant.CreateCollection{
		CollectionName: name,
		VectorsConfig: &qdrant.VectorsConfig{
			Config: &qdrant.VectorsConfig_Params{
				Params: &qdrant.VectorParams{
					Size:     vectorSize,
					Distance: qdrant.Distance_Cosine,
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create collection %s: %w", name, err)
	}

	return nil
}

// Upsert insere ou atualiza pontos
func (q *QdrantClient) Upsert(
	ctx context.Context,
	collectionName string,
	points []*qdrant.PointStruct,
) error {
	_, err := q.points.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: collectionName,
		Points:         points,
	})

	if err != nil {
		return fmt.Errorf("failed to upsert points: %w", err)
	}

	log.Printf("📥 [QDRANT] Upsert concluído: Coleção=%s, Pontos=%d", collectionName, len(points))
	return nil
}

// Search busca vetores similares
func (q *QdrantClient) Search(
	ctx context.Context,
	collectionName string,
	vector []float32,
	limit uint64,
	filter *qdrant.Filter,
) ([]*qdrant.ScoredPoint, error) {
	result, err := q.points.Search(ctx, &qdrant.SearchPoints{
		CollectionName: collectionName,
		Vector:         vector,
		Limit:          limit,
		Filter:         filter,
		WithPayload: &qdrant.WithPayloadSelector{
			SelectorOptions: &qdrant.WithPayloadSelector_Enable{
				Enable: true,
			},
		},
	})

	if err != nil {
		return nil, fmt.Errorf("failed to search: %w", err)
	}

	log.Printf("🔍 [QDRANT] Busca concluída: Coleção=%s, Resultados=%d", collectionName, len(result.Result))
	return result.Result, nil
}

// SearchWithScore busca com score mínimo e filtro de usuário
func (q *QdrantClient) SearchWithScore(
	ctx context.Context,
	collectionName string,
	vector []float32,
	limit uint64,
	minScore float32,
	userID int64,
) ([]*qdrant.ScoredPoint, error) {
	// Criar filtro para user_id
	filter := &qdrant.Filter{
		Must: []*qdrant.Condition{
			{
				ConditionOneOf: &qdrant.Condition_Field{
					Field: &qdrant.FieldCondition{
						Key: "user_id",
						Match: &qdrant.Match{
							MatchValue: &qdrant.Match_Integer{
								Integer: userID,
							},
						},
					},
				},
			},
		},
	}

	// Buscar
	result, err := q.Search(ctx, collectionName, vector, limit, filter)
	if err != nil {
		return nil, err
	}

	// Filtrar por score mínimo
	filtered := []*qdrant.ScoredPoint{}
	for _, point := range result {
		if point.Score >= minScore {
			filtered = append(filtered, point)
		}
	}

	return filtered, nil
}

// Delete remove pontos
func (q *QdrantClient) Delete(
	ctx context.Context,
	collectionName string,
	pointIDs []uint64,
) error {
	_, err := q.points.Delete(ctx, &qdrant.DeletePoints{
		CollectionName: collectionName,
		Points: &qdrant.PointsSelector{
			PointsSelectorOneOf: &qdrant.PointsSelector_Points{
				Points: &qdrant.PointsIdsList{
					Ids: convertToPointIDs(pointIDs),
				},
			},
		},
	})

	if err != nil {
		return fmt.Errorf("failed to delete points: %w", err)
	}

	return nil
}

// GetCollectionInfo retorna informações da coleção
func (q *QdrantClient) GetCollectionInfo(
	ctx context.Context,
	collectionName string,
) (*qdrant.CollectionInfo, error) {
	result, err := q.client.Get(ctx, &qdrant.GetCollectionInfoRequest{
		CollectionName: collectionName,
	})

	if err != nil {
		return nil, fmt.Errorf("failed to get collection info: %w", err)
	}

	return result.Result, nil
}

// Helper: converter IDs para PointId
func convertToPointIDs(ids []uint64) []*qdrant.PointId {
	result := make([]*qdrant.PointId, len(ids))
	for i, id := range ids {
		result[i] = &qdrant.PointId{
			PointIdOptions: &qdrant.PointId_Num{Num: id},
		}
	}
	return result
}

// Helper: criar ponto
func CreatePoint(
	id uint64,
	vector []float32,
	payload map[string]interface{},
) *qdrant.PointStruct {
	// Converter payload para Qdrant Value
	qdrantPayload := make(map[string]*qdrant.Value)
	for key, val := range payload {
		qdrantPayload[key] = toQdrantValue(val)
	}

	return &qdrant.PointStruct{
		Id: &qdrant.PointId{
			PointIdOptions: &qdrant.PointId_Num{Num: id},
		},
		Vectors: &qdrant.Vectors{
			VectorsOptions: &qdrant.Vectors_Vector{
				Vector: &qdrant.Vector{Data: vector},
			},
		},
		Payload: qdrantPayload,
	}
}

// Helper: converter valor Go para Qdrant Value
func toQdrantValue(val interface{}) *qdrant.Value {
	switch v := val.(type) {
	case int:
		return &qdrant.Value{
			Kind: &qdrant.Value_IntegerValue{IntegerValue: int64(v)},
		}
	case int64:
		return &qdrant.Value{
			Kind: &qdrant.Value_IntegerValue{IntegerValue: v},
		}
	case float64:
		return &qdrant.Value{
			Kind: &qdrant.Value_DoubleValue{DoubleValue: v},
		}
	case string:
		return &qdrant.Value{
			Kind: &qdrant.Value_StringValue{StringValue: v},
		}
	case bool:
		return &qdrant.Value{
			Kind: &qdrant.Value_BoolValue{BoolValue: v},
		}
	default:
		return &qdrant.Value{
			Kind: &qdrant.Value_StringValue{StringValue: fmt.Sprintf("%v", v)},
		}
	}
}
