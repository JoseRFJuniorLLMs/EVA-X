// migration_voice.cypher
// ─────────────────────────────────────────────────────────────────────────────
// Migração Neo4j para o módulo de Biometria de Voz da EVA
// Execute no Neo4j Browser ou via cypher-shell:
//   cypher-shell -u neo4j -p <senha> < migration_voice.cypher
// ─────────────────────────────────────────────────────────────────────────────

// ── 1. Constraints ────────────────────────────────────────────────────────
CREATE CONSTRAINT person_id IF NOT EXISTS
  FOR (p:Person) REQUIRE p.id IS UNIQUE;

CREATE CONSTRAINT voice_profile_id IF NOT EXISTS
  FOR (v:VoiceProfile) REQUIRE v.id IS UNIQUE;

CREATE CONSTRAINT voice_event_id IF NOT EXISTS
  FOR (e:VoiceEvent) REQUIRE e.id IS UNIQUE;

// ── 2. Índices ────────────────────────────────────────────────────────────
// Otimiza a query de carregamento de perfis ativos (usada a cada cache miss)
CREATE INDEX voice_profile_active IF NOT EXISTS
  FOR (v:VoiceProfile) ON (v.active);

CREATE INDEX voice_profile_speaker IF NOT EXISTS
  FOR (v:VoiceProfile) ON (v.speaker_id);

// Índice temporal para análise de eventos de voz (Hebbian log)
CREATE INDEX voice_event_timestamp IF NOT EXISTS
  FOR (e:VoiceEvent) ON (e.timestamp);

// ── 3. Nós Person (exemplo — ajuste para os seus cadastros) ───────────────
MERGE (p1:Person {id: 'person_junior'})
  SET p1.name = 'Junior', p1.created_at = datetime(), p1.active = true;

MERGE (p2:Person {id: 'person_coraline'})
  SET p2.name = 'Coraline', p2.created_at = datetime(), p2.active = true;

MERGE (p3:Person {id: 'person_elizabeth'})
  SET p3.name = 'Elizabeth', p3.created_at = datetime(), p3.active = true;

// ── 4. Estrutura do Grafo de Voz ──────────────────────────────────────────
//
// (Person) -[:HAS_VOICE_PROFILE {confidence, created_at, last_updated}]->
//     (VoiceProfile {id, speaker_id, centroid[512], intra_variance,
//                    sample_count, enrolled_at, active, last_seen,
//                    recognition_count})
//
// (Person) -[:VOICE_EVENT]->
//     (VoiceEvent {id, cosine_sim, residual_err, confidence,
//                  confirmed, audio_quality, timestamp})
//
// Nota: O campo `centroid` é um array de 512 floats armazenado diretamente
// no nó. O Neo4j suporta arrays de primitivos nativamente e os retorna
// eficientemente — não é necessário serializar como string.

// ── 5. Query de leitura (usada pelo Go a cada cache miss) ─────────────────
//
// MATCH (p:Person)-[:HAS_VOICE_PROFILE]->(v:VoiceProfile {active: true})
// RETURN
//   p.id                    AS speaker_id,
//   p.name                  AS name,
//   v.centroid              AS centroid,
//   v.intra_variance        AS variance,
//   v.sample_count          AS samples,
//   toString(v.enrolled_at) AS enrolled_at
// ORDER BY p.name

// ── 6. Diagnóstico: confiança atual de todos os perfis ────────────────────
//
// MATCH (p:Person)-[r:HAS_VOICE_PROFILE]->(v:VoiceProfile {active: true})
// RETURN
//   p.name                AS name,
//   r.confidence          AS link_confidence,
//   v.intra_variance      AS variance,
//   v.sample_count        AS samples,
//   v.recognition_count   AS total_recognitions,
//   toString(v.last_seen) AS last_seen
// ORDER BY r.confidence DESC

// ── 7. Diagnóstico: últimos 20 eventos de voz de um falante ───────────────
//
// MATCH (p:Person {name: 'Junior'})-[:VOICE_EVENT]->(e:VoiceEvent)
// RETURN
//   e.cosine_sim     AS cosine,
//   e.residual_err   AS residual,
//   e.confidence     AS confidence,
//   e.confirmed      AS confirmed,
//   e.audio_quality  AS quality_db,
//   toString(e.timestamp) AS ts
// ORDER BY e.timestamp DESC
// LIMIT 20

// ── 8. Reset de perfil (útil durante testes) ──────────────────────────────
//
// MATCH (p:Person {id: 'person_junior'})-[r:HAS_VOICE_PROFILE]->(v:VoiceProfile)
// SET r.confidence = 1.0, r.last_updated = datetime()

// ── 9. Desativar perfil (ex: após mudança vocal permanente) ───────────────
//
// MATCH (v:VoiceProfile {speaker_id: 'person_junior'})
// SET v.active = false
// // Em seguida, rode novo enroll e UpsertProfile via API
