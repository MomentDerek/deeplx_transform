/translate
Method: POST
Request Headers
Header	Description	Value
Content-Type	The content type of the request body.	application/json
Authorization	The access token to protect your API.	Bearer your_access_token
Please note that if you are unable to pass Authorization, you can also use URL Params instead. For example, /translate?token=your_access_token

Request Parameters
Parameter	Type	Description	Required
text	string	The text you want to translate.	true
source_lang	string	The language code of the source text.	true
target_lang	string	The language code you want to translate to.	true
Response
{
"alternatives": [
"Did you hear about this?",
"You've heard about this?",
"You've heard of this?"
],
"code": 200,
"data": "Have you heard about this?",
"id": 8356681003,
"method": "Free",
"source_lang": "ZH",
"target_lang": "EN"
}

/v2/translate
Method: POST
Request Headers
Header	Description	Value
Content-Type	The content type of the request body.	application/json
Authorization	The authorization of the request.	Authorization: DeepL-Auth-Key [yourAccessToken]
Please note that if you want to pass two parameters at the same time, separate them with a space.

Request Parameters
Parameter	Type	Description	Required
text	string	The text you want to translate.	true
source_lang	string	The language code of the source text.	false
target_lang	string	The language code you want to translate to.	true

response
{
"translations": [
{
"detected_source_language": "EN",
"text": "Hallo, Welt!"
}
]
}