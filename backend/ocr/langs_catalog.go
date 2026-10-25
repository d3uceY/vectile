package ocr

import "sort"

// Language is one entry a user can pick in Settings → OCR. Code is what goes
// into config, Name is what a person reads.
type Language struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

// languages is every language the pinned tessdata_fast commit offers, in the
// spelling that repository uses. Deliberately excludes the non-language files
// it also ships: osd (orientation and script detection), equ (equations), frk
// (a deprecated alias of deu_latf) and the script/ models, which cover a whole
// writing system and are not something a person picks per language.
//
// A static table rather than a network call: the picker has to work offline,
// and the list only changes when tessdataBase does.
var languages = []Language{
	{"afr", "Afrikaans"},
	{"sqi", "Albanian"},
	{"amh", "Amharic"},
	{"ara", "Arabic"},
	{"hye", "Armenian"},
	{"asm", "Assamese"},
	{"aze", "Azerbaijani"},
	{"aze_cyrl", "Azerbaijani (Cyrillic)"},
	{"eus", "Basque"},
	{"bel", "Belarusian"},
	{"ben", "Bengali"},
	{"bos", "Bosnian"},
	{"bre", "Breton"},
	{"bul", "Bulgarian"},
	{"mya", "Burmese"},
	{"cat", "Catalan"},
	{"ceb", "Cebuano"},
	{"chr", "Cherokee"},
	{"chi_sim", "Chinese (Simplified)"},
	{"chi_sim_vert", "Chinese (Simplified, vertical)"},
	{"chi_tra", "Chinese (Traditional)"},
	{"chi_tra_vert", "Chinese (Traditional, vertical)"},
	{"cos", "Corsican"},
	{"hrv", "Croatian"},
	{"ces", "Czech"},
	{"dan", "Danish"},
	{"div", "Dhivehi"},
	{"nld", "Dutch"},
	{"dzo", "Dzongkha"},
	{"eng", "English"},
	{"enm", "English (Middle)"},
	{"epo", "Esperanto"},
	{"est", "Estonian"},
	{"fao", "Faroese"},
	{"fil", "Filipino"},
	{"fin", "Finnish"},
	{"fra", "French"},
	{"frm", "French (Middle)"},
	{"fry", "Frisian"},
	{"glg", "Galician"},
	{"kat", "Georgian"},
	{"kat_old", "Georgian (Old)"},
	{"deu", "German"},
	{"deu_latf", "German (Fraktur)"},
	{"ell", "Greek"},
	{"grc", "Greek (Ancient)"},
	{"guj", "Gujarati"},
	{"hat", "Haitian Creole"},
	{"heb", "Hebrew"},
	{"hin", "Hindi"},
	{"hun", "Hungarian"},
	{"isl", "Icelandic"},
	{"ind", "Indonesian"},
	{"iku", "Inuktitut"},
	{"gle", "Irish"},
	{"ita", "Italian"},
	{"ita_old", "Italian (Old)"},
	{"jpn", "Japanese"},
	{"jpn_vert", "Japanese (vertical)"},
	{"jav", "Javanese"},
	{"kan", "Kannada"},
	{"kaz", "Kazakh"},
	{"khm", "Khmer"},
	{"kir", "Kyrgyz"},
	{"kor", "Korean"},
	{"kor_vert", "Korean (vertical)"},
	{"kmr", "Kurdish (Kurmanji)"},
	{"lao", "Lao"},
	{"lat", "Latin"},
	{"lav", "Latvian"},
	{"lit", "Lithuanian"},
	{"ltz", "Luxembourgish"},
	{"mkd", "Macedonian"},
	{"msa", "Malay"},
	{"mal", "Malayalam"},
	{"mlt", "Maltese"},
	{"mri", "Maori"},
	{"mar", "Marathi"},
	{"mon", "Mongolian"},
	{"nep", "Nepali"},
	{"nor", "Norwegian"},
	{"oci", "Occitan"},
	{"ori", "Odia"},
	{"pus", "Pashto"},
	{"fas", "Persian"},
	{"pol", "Polish"},
	{"por", "Portuguese"},
	{"pan", "Punjabi"},
	{"que", "Quechua"},
	{"ron", "Romanian"},
	{"rus", "Russian"},
	{"san", "Sanskrit"},
	{"gla", "Scottish Gaelic"},
	{"srp", "Serbian"},
	{"srp_latn", "Serbian (Latin)"},
	{"snd", "Sindhi"},
	{"sin", "Sinhala"},
	{"slk", "Slovak"},
	{"slv", "Slovenian"},
	{"spa", "Spanish"},
	{"spa_old", "Spanish (Old)"},
	{"sun", "Sundanese"},
	{"swa", "Swahili"},
	{"swe", "Swedish"},
	{"syr", "Syriac"},
	{"tgk", "Tajik"},
	{"tam", "Tamil"},
	{"tat", "Tatar"},
	{"tel", "Telugu"},
	{"tha", "Thai"},
	{"bod", "Tibetan"},
	{"tir", "Tigrinya"},
	{"ton", "Tongan"},
	{"tur", "Turkish"},
	{"uig", "Uyghur"},
	{"ukr", "Ukrainian"},
	{"urd", "Urdu"},
	{"uzb", "Uzbek"},
	{"uzb_cyrl", "Uzbek (Cyrillic)"},
	{"vie", "Vietnamese"},
	{"cym", "Welsh"},
	{"yid", "Yiddish"},
	{"yor", "Yoruba"},
}

// Catalog returns every language a user can add, English first and the rest by
// display name. Sorted on every call rather than stored sorted: the list is
// hand-maintained and a misplaced line should not reach the UI as a stray row.
func Catalog() []Language {
	out := make([]Language, len(languages))
	copy(out, languages)
	sort.SliceStable(out, func(i, j int) bool {
		if (out[i].Code == EnglishCode) != (out[j].Code == EnglishCode) {
			return out[i].Code == EnglishCode
		}
		return out[i].Name < out[j].Name
	})
	return out
}
