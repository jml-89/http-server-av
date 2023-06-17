package main

// DESIGN#2
// In order for this whole thing to compile neatly into one binary with no file dependencies
// The templates, json config, etc. are all here in string literals
// This is fine (really?) for deployment but for development this is annoying
// Having to recompile every time you want to see changes in HTML/config reflect

var starterTemplates = map[string]string{
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

		#templates {
			display: flex;
			flex-direction: column;
		}

		pre {
			font: 100% sans-serif;
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

			<form id="search-area" action="/search" method="get">
				<input type="text" name="terms" id="search-terms" value="{{.terms}}" required>
				<input type="submit" value="Search">
			</form>
		</div>
		{{template "body" .}}
	</body>
</html>
`,
	"templates": `
<div class="templates">
{{range $idx, $elem := .elements}}
	<form id="form-{{index $elem 0}}" action="templates" method="post">
		<h2>{{index $elem 0}}</h2>
		<textarea name="raw">{{index $elem 1}}</textarea>
		<input type="hidden" name="name" value="{{index $elem 0}}">
		<input type="hidden" name="previous" value="{{index $elem 2}}">
		<input type="submit" value="Update">
	</form>
{{end}}
</div>
`,
	"video": `
<div>
	<video controls>
		<source src="/file/{{index .filename 0 | escapepath}}">
		<a href="/file/{{index .filename 0 | escapepath}}">Download</a>
	</video>

	<div id="video-description">
		<h1>{{if .title}}{{index .title 0}}{{else}}{{index .filename 0}}{{end}}</h1>
		{{if .artist}}<h2>Created by <a href="/search?terms=artist:&quot;{{index .artist 0 }}&quot;">{{index .artist 0}}</a></h2>{{end}}
	</div>

	{{if .favourite}}
	<form id="remove-from-favourites" action="/favourites/remove" method="post">
		<input type="hidden" name="filename" value="{{index .filename 0}}">
		<input type="submit" value="Remove from Favourites">
	</form>
	{{else}}
	<form id="add-to-favourites" action="/favourites/add" method="post">
		<input type="hidden" name="filename" value="{{index .filename 0}}">
		<input type="submit" value="Add to Favourites">
	</form>
	{{end}}


	<h1>Metadata</h1>
	<table>
		<tbody>
		{{range $k, $pair := .tags}}
			<tr>
				<td><a href="/search?terms={{index $pair 0}}:&quot;{{index $pair 1}}&quot;">{{index $pair 0}}</a></td>
				<td>{{index $pair 1}}</td>
			</tr>
		{{end}}
		</tbody>
	</table>

	<h1>Related Videos</h1>
	<ol>{{range $k, $vs := .related}}
		<li><a href="/watch?filename={{index $vs 0 | escapequery}}"><img src="/tmb/{{index $vs 1 | escapepath}}"/><h2>{{index $vs 0}}</h2></a></li>
	{{end}}</ol>
</div>
`,
	"index": `
<div id="thumbs">
{{range $idx, $elem := .videos}}
	<div class="media-item">
		<a href="/watch?filename={{index $elem 0 | escapequery}}"><img src="/tmb/{{index $elem 1 | escapepath}}"/>
		<h2>{{index $elem 0}}</h2></a>
	</div>
{{end}}
</div>
`,
	"listing": `
<form action="{{.path}}" method="post">
<ol>
	{{range $idx, $elem := .elements}}
	<li>
		<input type="checkbox" name="elements" value="{{.}}">
		<a href="/search?terms={{. | escapequery}}">{{.}}</a>
	</li>
	{{end}}
</ol>
<button type="submit">Remove</button>
</form>
`,
	"single": `
<html>
	<head>
		<title>{{.}}</title>
	</head>
	<body>
		<h1>{{.}}</h1>
		<video controls>
			<source src="/file/{{. | escapepath}}"/>
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
`,
	"init": `
[
	"create table if not exists favourites (filename text, primary key (filename));"
]
`,
	"routes": `
{
	"/": {
		"get": {
			"template": "index",
			"items": {
				"constant": {
					"count": "50"
				},
				"query": {
					"videos": "select a.filename, b.val from tags a join tags b on a.filename = b.filename where a.name is 'diskfiletime' and b.name is 'thumbname' order by a.val desc limit 50;" 
				}
			}
		}
	},

	"/favourites/": {
		"get": {
			"template": "index",
			"items": {
				"query": {
					"videos": "select a.filename, a.val from favourites b join tags a on a.filename = b.filename where a.name is 'thumbname';"
				}
			}
		}
	},

	"/favourites/add": {
		"post": {
			"query": "insert or ignore into favourites (filename) values (:filename);",
			"redirect": "/favourites/"
		}
	},

	"/favourites/remove": {
		"post": {
			"query": "delete from favourites where filename is :filename;",
			"redirect": "/favourites/"
		}
	},

	"/watch": {
		"get": {
			"template": "video",
			"items": {
				"query": {
					"filename": "select distinct(filename) from tags where filename is :filename;",
					"title": "select val from tags where filename is :filename and name is 'title';",
					"artist": "select val from tags where filename is :filename and name is 'artist';",
					"tags": "select name, val from tags where filename is :filename;",
					"related": "select filename, val from tags where name is 'thumbname' and filename in (select filename from wordassocs a where a.word in (select word from wordassocs where filename is :filename) group by a.filename order by (count(a.filename) * 100) / (select count(*) from wordassocs where filename is a.filename) desc limit 10);",
					"favourite": "select filename from favourites where filename is :filename;"
				}
			}
		}
	},

	"/templates/": {
		"get": {
			"template": "templates",
			"items": {
				"query": {
					"elements": "select name, raw, rowid from templates where rowid not in (select previous from templates);"
				}
			}
		},
		"post": {
			"query": "insert or ignore into templates (previous, name, raw) values (:previous, :name, :raw);",
			"redirect": "/templates/"
		}
	}
}
`,
}

var Fastlinks []Route = []Route{
	{Path: "/", Alias: "Home"},
	{Path: "/favourites/", Alias: "Favourites"},
	{Path: "/templates/", Alias: "Templates"},
}
