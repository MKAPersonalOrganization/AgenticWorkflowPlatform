// Package prompt provides prompt templates for LightRAG LLM operations.
package prompt

// DefaultTupleDelimiter is the delimiter for tuple fields in extraction.
const DefaultTupleDelimiter = "<|#|>"

// DefaultCompletionDelimiter indicates extraction completion.
const DefaultCompletionDelimiter = "<|COMPLETE|>"

// Prompts is a map containing all prompt templates.
var Prompts = map[string]interface{}{
	"DEFAULT_TUPLE_DELIMITER":      DefaultTupleDelimiter,
	"DEFAULT_COMPLETION_DELIMITER": DefaultCompletionDelimiter,
}

// EntityExtractionSystemPrompt is the system prompt for entity extraction.
const EntityExtractionSystemPrompt = `---Role---
You are a Knowledge Graph Specialist responsible for extracting entities and relationships from the input text.

---Instructions---
1.  **Entity Extraction & Output:**
    *   **Identification:** Identify clearly defined and meaningful entities in the input text.
    *   **Entity Details:** For each identified entity, extract the following information:
        *   ` + "`entity_name`" + `: The name of the entity. If the entity name is case-insensitive, capitalize the first letter of each significant word (title case). Ensure **consistent naming** across the entire extraction process.
        *   ` + "`entity_type`" + `: Categorize the entity using one of the following types: ` + "`{entity_types}`" + `. If none of the provided entity types apply, do not add new entity type and classify it as ` + "`Other`" + `.
        *   ` + "`entity_description`" + `: Provide a concise yet comprehensive description of the entity's attributes and activities, based *solely* on the information present in the input text.
    *   **Output Format - Entities:** Output a total of 4 fields for each entity, delimited by ` + "`{tuple_delimiter}`" + `, on a single line. The first field *must* be the literal string ` + "`entity`" + `.
        *   Format: ` + "`entity{tuple_delimiter}entity_name{tuple_delimiter}entity_type{tuple_delimiter}entity_description`" + `

2.  **Relationship Extraction & Output:**
    *   **Identification:** Identify direct, clearly stated, and meaningful relationships between previously extracted entities.
    *   **N-ary Relationship Decomposition:** If a single statement describes a relationship involving more than two entities (an N-ary relationship), decompose it into multiple binary (two-entity) relationship pairs for separate description.
    *   **Relationship Details:** For each binary relationship, extract the following fields:
        *   ` + "`source_entity`" + `: The name of the source entity.
        *   ` + "`target_entity`" + `: The name of the target entity.
        *   ` + "`relationship_keywords`" + `: One or more high-level keywords summarizing the relationship.
        *   ` + "`relationship_description`" + `: A concise explanation of the nature of the relationship.
    *   **Output Format - Relationships:** Output a total of 5 fields for each relationship, delimited by ` + "`{tuple_delimiter}`" + `, on a single line. The first field *must* be the literal string ` + "`relation`" + `.
        *   Format: ` + "`relation{tuple_delimiter}source_entity{tuple_delimiter}target_entity{tuple_delimiter}relationship_keywords{tuple_delimiter}relationship_description`" + `

3.  **Delimiter Usage Protocol:**
    *   The ` + "`{tuple_delimiter}`" + ` is a complete, atomic marker and **must not be filled with content**.

4.  **Relationship Direction & Duplication:**
    *   Treat all relationships as **undirected** unless explicitly stated otherwise.
    *   Avoid outputting duplicate relationships.

5.  **Output Order & Prioritization:**
    *   Output all extracted entities first, followed by all extracted relationships.

6.  **Context & Objectivity:**
    *   Ensure all entity names and descriptions are written in the **third person**.
    *   Explicitly name the subject or object; **avoid using pronouns**.

7.  **Language & Proper Nouns:**
    *   The entire output must be written in ` + "`{language}`" + `.
    *   Proper nouns should be retained in their original language.

8.  **Completion Signal:** Output the literal string ` + "`{completion_delimiter}`" + ` only after all entities and relationships have been completely extracted.

---Examples---
{examples}
`

// EntityExtractionUserPrompt is the user prompt for entity extraction.
const EntityExtractionUserPrompt = `---Task---
Extract entities and relationships from the input text in Data to be Processed below.

---Instructions---
1.  **Strict Adherence to Format:** Strictly adhere to all format requirements for entity and relationship lists.
2.  **Output Content Only:** Output *only* the extracted list of entities and relationships.
3.  **Completion Signal:** Output ` + "`{completion_delimiter}`" + ` as the final line.
4.  **Output Language:** Ensure the output language is {language}.

---Data to be Processed---
<Entity_types>
[{entity_types}]

<Input Text>
` + "```" + `
{input_text}
` + "```" + `

<Output>
`

// EntityContinueExtractionUserPrompt is used to continue extraction for missed entities.
const EntityContinueExtractionUserPrompt = `---Task---
Based on the last extraction task, identify and extract any **missed or incorrectly formatted** entities and relationships from the input text.

---Instructions---
1.  **Strict Adherence to System Format:** Strictly adhere to all format requirements.
2.  **Focus on Corrections/Additions:**
    *   **Do NOT** re-output entities and relationships that were correctly extracted.
    *   If an entity or relationship was missed, extract and output it now.
    *   If something was incorrectly formatted, re-output the corrected version.
3.  **Output Format - Entities:** Output a total of 4 fields for each entity, delimited by ` + "`{tuple_delimiter}`" + `.
4.  **Output Format - Relationships:** Output a total of 5 fields for each relationship, delimited by ` + "`{tuple_delimiter}`" + `.
5.  **Output Content Only:** Output *only* the extracted list.
6.  **Completion Signal:** Output ` + "`{completion_delimiter}`" + ` as the final line.
7.  **Output Language:** Ensure the output language is {language}.

<Output>
`

// SummarizeEntityDescriptionsPrompt is used for summarizing entity descriptions.
const SummarizeEntityDescriptionsPrompt = `---Role---
You are a Knowledge Graph Specialist, proficient in data curation and synthesis.

---Task---
Your task is to synthesize a list of descriptions of a given entity or relation into a single, comprehensive, and cohesive summary.

---Instructions---
1. Input Format: The description list is provided in JSON format.
2. Output Format: The merged description will be returned as plain text, presented in multiple paragraphs.
3. Comprehensiveness: The summary must integrate all key information from *every* provided description.
4. Context: Ensure the summary is written from an objective, third-person perspective.
5. Conflict Handling: In cases of conflicting descriptions, determine if they arise from multiple distinct entities sharing the same name.
6. Language: Output in {language}. Keep proper nouns in their original language.
7. Length: Keep the summary concise, around {summary_max_tokens} tokens.

---Description List---
{description_list}

---Output---
`

// RAGResponsePrompt is the prompt for generating RAG responses.
const RAGResponsePrompt = `---Role---
You are a helpful assistant responding to questions about data in the tables provided.

---Goal---
Generate a response of the target length and format that responds to the user's question, summarizing all information in the input data tables appropriate for the response length and format, and incorporating any relevant general knowledge.
If you don't know the answer, just say so. Do not make anything up.
Do not include information where the supporting evidence for it is not provided.

---Target response length and format---
{response_type}

Add sections and commentary to the response as appropriate for the length and format. Style the response in markdown.

---Data tables---

{context_data}

---Goal---
Generate a response of the target length and format that responds to the user's question, summarizing all information in the input data tables appropriate for the response length and format, and incorporating any relevant general knowledge.

If you don't know the answer, just say so. Do not make anything up.
Do not include information where the supporting evidence for it is not provided.

---Target response length and format---
{response_type}

Add sections and commentary to the response as appropriate for the length and format. Style the response in markdown.
`

// NaiveRAGResponsePrompt is the prompt for naive RAG responses.
const NaiveRAGResponsePrompt = `You are a helpful assistant responding to questions about documents provided.

---Documents---

{content_data}

---Goal---
Generate a response that answers the user's question based on the documents provided.
If you don't know the answer, just say so. Do not make anything up.

Response format: {response_type}
`

// KeywordExtractionPrompt is used for extracting keywords from queries.
const KeywordExtractionPrompt = `---Role---
You are a helpful assistant tasked with identifying both high-level and low-level keywords in the user's query.

---Goal---
Given the query, list high-level and low-level keywords. High-level keywords focus on overarching concepts or themes, while low-level keywords focus on specific entities, details, or concrete terms.

---Instructions---
- Output the keywords in JSON format.
- The JSON should have two keys:
  - "high_level_keywords" for overarching concepts or themes
  - "low_level_keywords" for specific entities or details

######################
-Examples-
######################
Example 1:

Query: "How does international trade influence global economic stability?"
################
Output:
{{
  "high_level_keywords": ["International trade", "Global economic stability", "Economic influence"],
  "low_level_keywords": ["Trade agreements", "Tariffs", "Currency exchange", "Imports", "Exports"]
}}
#############################
Example 2:

Query: "What are the environmental consequences of deforestation on biodiversity?"
################
Output:
{{
  "high_level_keywords": ["Environmental consequences", "Deforestation", "Biodiversity loss"],
  "low_level_keywords": ["Species extinction", "Habitat destruction", "Carbon emissions", "Rainforest", "Ecosystem"]
}}
#############################
Example 3:

Query: "What is the role of education in reducing poverty?"
################
Output:
{{
  "high_level_keywords": ["Education", "Poverty reduction", "Socioeconomic development"],
  "low_level_keywords": ["School access", "Literacy rates", "Job training", "Income inequality"]
}}
#############################
-Real Data-
######################
Query: {query}
######################
Output:
`

// FailPrompt is returned when extraction fails.
const FailPrompt = "Sorry, extraction failed. Please check the input and try again."

func init() {
	// Register all prompts
	Prompts["entity_extraction_system_prompt"] = EntityExtractionSystemPrompt
	Prompts["entity_extraction_user_prompt"] = EntityExtractionUserPrompt
	Prompts["entity_continue_extraction_user_prompt"] = EntityContinueExtractionUserPrompt
	Prompts["summarize_entity_descriptions"] = SummarizeEntityDescriptionsPrompt
	Prompts["rag_response"] = RAGResponsePrompt
	Prompts["naive_rag_response"] = NaiveRAGResponsePrompt
	Prompts["keyword_extraction"] = KeywordExtractionPrompt
	Prompts["fail_response"] = FailPrompt
}

// GetPrompt retrieves a prompt by name.
func GetPrompt(name string) (string, bool) {
	val, ok := Prompts[name]
	if !ok {
		return "", false
	}
	str, ok := val.(string)
	return str, ok
}
