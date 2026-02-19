New Requirement

Currenlty the llm model and embedding model configurations are shared across all users.  We want to switch off this model
and allow users to enter their api keys. SO the user takes the responsiblity of the cost. 


- As a user I want to add a new llm provider (openiai, anthropic,...)
- As a user i want to select one model provider as default.
- As a user i can set the api key needed by that model provider
- As a  system i can show for the user a list of models the model provider supports
- As a user i can selelct a defulat LLM model for that provider
- As a user i can select a defulat embedding model for that provider, 
- As a user i can choose the settings for the model (max tokens,temperature, embedding dimensions,...) but those are behind advanced features.


We need a new AISettings Service for that. which stores these user settings for each  model in  postgres table. 
Each user is comploety seperate from all other users and isolated. 

The AI settings service is wrapped by a handlers layer for access to he frontend


The AI settings service is used by all services that needs to get credentials


We need to remove the support for shared LLM across all users, and all related code.  no need to be backward compatbile.


In the froned craate a new form under settings in the navbar uner (General, AI, Personalization, Data), the new feature will be under AI. 


Please don't stop untill you complete implement the feature across all layers 
and the application builds successfully. 



--- 