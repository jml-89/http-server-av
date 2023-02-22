package main

// DESIGN#2
// In order for this whole thing to compile neatly into one binary with no file dependencies
// The templates, json config, etc. are all here in string literals
// This is fine (really?) for deployment but for development this is annoying
// Having ot recompile every time you want to see changes in HTML/config reflect

var templates = map[string]string{
	"base": `
<html>
	<head>
		<title>Typing At Computers</title>
	</head>
	<style>
		#thumbs {
			display: flex;
			flex-direction: column;
			align-items: stretch;
			justify-content: stretch;

			background: #338888;
		}

		#top-bar {
			display: flex;
			flex-direction: row;
			align-items: stretch;
		}

		#search-area {
			margin-left: auto;
		}

		#search-terms {
			width: 35rem;
		}

		.media-item {
			margin: .5rem .5rem 0rem .5rem;
			padding: 1rem;
			background: #AACCAA;

			display: flex;
			flex-direction: column;
			align-items: center;
			justify-content: center;
		}

		img,
		picture,
		video,
		table {
			max-width: 100%;
		}
	</style>
	<body>
		<div id="top-bar">
			<div id="quick-links">
				{{range $i, $e := .routes}}
				<a href="{{.Path}}">{{.Alias}}</a>
				{{end}}
			</div>

			<form id="search-area" action="search" method="get">
				<input type="text" name="terms" id="search-terms" required>
				<input type="submit" value="Search">
			</form>
		</div>
		{{template "body" .}}
	</body>
</html>
`,
"video":`
<div>
{{range $idx, $elem := .media}}
	<video controls>
		<source src="/file/{{$elem.filename}}">
		<a href="/file/{{$elem.filename}}">Download</a>
	</video>
	<div id="video-description">
		<h1>{{$elem.title}}</h1>
		<h2>Created by <a href="/search?terms=artist:&quot;{{$elem.artist}}&quot;">{{$elem.artist}}</a></h2>
		<h3>Uploaded on {{$elem.date}}</h3>
		<pre>{{$elem.description}}</pre>
	</div>

	<h1>Metadata</h1>
	<table>
		<tbody>
		{{range $k, $v := $elem}}
			<tr>
				<td><a href="/search?terms={{$k}}:&quot;{{$v}}&quot;">{{$k}}</a></td>
				<td>{{$v}}</td>
			</tr>
		{{end}}
		</tbody>
	</table>
{{end}}
</div>
`,
"index":`
<div id="thumbs">
{{range $idx, $elem := .media}}
	<div class="media-item">
		<a href="/watch?arg={{$elem.filename}}"><img src="/tmb/{{$elem.thumbname}}"/></a>
		<a href="/watch?arg={{$elem.filename}}"><h2>{{$elem.diskfilename}}</h2></a>
	</div>
{{end}}
</div>
`,
"listing":`
<form action="{{.path}}" method="post">
<ol>
	{{range $idx, $elem := .elements}}
	<li>
		<input type="checkbox" name="elements" value="{{.}}">
		<a href="/search?terms={{.}}">{{.}}</a>
	</li>
	{{end}}
</ol>
<button type="submit">Remove</button>
</form>
`,
"single":`
<html>
	<head>
		<title>{{.}}</title>
	</head>
	<body>
		<h1>{{.}}</h1>
		<video controls>
			<source src="/file/{{.}}"/>
		</video>
	</body>

	<style>
		video {
			height: 90%;
			width: 90%;
			object-fit: cover;
			position: relative;
			display: block;
			margin-left: auto;
			margin-right: auto;
		}
	</style>
</html>
`}

var jsonRoutes string = `
{
	"/": {
		"get": {
			"template": "index",
			"items": {
				"constant": {
					"count": "50"
				},
				"query": {
					"media": "select distinct(filename) from tags where name is 'diskfiletime' order by val desc limit 50;"
				}
			}
		}
	},

	"/watch": {
		"get": {
			"template": "video",
			"items": {
				"query": {
					"media": "select distinct(filename) from tags where filename is ?;"
				}
			}
		}
	},

	"/top-words": {
		"get": {
			"template": "listing",
			"items": {
				"constant": {},
				"query": {
					"elements": "select word from wordcounts where blacklist = 0 order by num desc limit 50;"
				}
			}
		},
		"post": {
			"query": "update wordcounts set blacklist = 1 where word is ?;",
			"arg": "elements",
			"redirect": "/top-words"
		}
	},

	"/random-words": {
		"get": {
			"template": "listing",
			"items": {
				"constant": {},
				"query": {
					"elements": "select word from wordcounts where num > 10 and blacklist = 0 order by random() limit 50;"
				}
			}
		},
		"post": {
			"query": "update wordcounts set blacklist = 1 where word is ?;",
			"arg": "elements",
			"redirect": "/random-words"
		}
	},

	"/artists": {
		"get": {
			"template": "listing",
			"items": {
				"constant": {},
				"query": {
					"elements": "select distinct(val) from tags where name is 'artist' order by random();"
				}
			}
		}
	}
}
`
