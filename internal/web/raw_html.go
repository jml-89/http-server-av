package web

// DESIGN#2
// In order for this whole thing to compile neatly into one binary with no file dependencies
// The templates, json config, etc. are all here in string literals
// Then they get added to the database
// Can change them in there and see changes instantly
// Except for routes, they need to be reloaded

// HTML templates
// These are inserted into the database in table called 'templates'
var starterTemplates = map[string]string{
	"base": `
<!DOCTYPE html>
<html>
	<head>
		<style>
			body {
				display: flex;
				flex-direction: column;
				min-height: 100dvh;
				justify-content: flex-start;
				gap: 0.5rem;
			}

			.nav-bar {
				display: flex;
				flex-direction: row;
				gap: 1rem;

				font-size: 3rem;
			}

			.search-area {
				align-self: stretch;

				display: flex;
				flex-direction: row;
				justify-content: stretch;
				gap: 0.25rem;
			}

			.search-box {
				min-width: 5rem;
				flex: 1 1 0%;
				font-size: 3rem;
				border-radius: 1rem;
			}

			.big-button {
				font-size: 3rem;
				border-radius: 1rem;
			}

			.thumbs {
				display: flex;
				flex-direction: column;
				align-items: center;
				gap: 1.25rem;
			}

			.refinement-area {

				display: flex;
				flex-direction: row;
				justify-content: space-between;
				gap: 0.5rem;
			}

			.search-refinements {
				display: flex;
				flex-direction: column;
				gap: 0.5rem;
			}

			.search-refinements div {
				font-size: 2.5rem;
			}

			.search-refinements input {
				font-size: 2rem;
				border-radius: 0.5rem;
				padding: 0.25rem;
			}

			.search-refinement-words {
				flex: 1 1 0%;

				display: flex;
				flex-direction: column;
				gap: 0.5rem;
			}

			.refine-words {
				display: flex;
				flex-direction: row;
				flex-wrap: wrap;
				gap: 0.5rem;
			}

			.search-refinement-words div {
				font-size: 2.5rem;
			}

			.search-refinement-words input {
				font-size: 2rem;
				border-radius: 0.5rem;
				padding: 0.25rem;
			}

			.selected {
				font-weight: 700;
			}

			.media-item {
				padding: 0.25rem;

				display: flex;
				flex-direction: column;
				align-items: center;
			}

			.thumb-img {
				object-fit: contain;
			}

			.media-title {
				font-size: 1.75rem;
				font-weight: 300;
			}

			pre {
				font: 100% sans-serif;
			}

			.flexy-col {
				display: flex;
				flex-direction: column;
				gap: 0.25rem;
				align-items: baseline;
			}

			.stretchy-item {
				align-self: stretch;
			}

			.mega-flexy-col {
				flex: 1 1 0%;

				display: flex;
				flex-direction: column;
				justify-content: flex-end;
				gap: 0.25rem;
				align-items: baseline;
			}

			.big-text-box {
				align-self: stretch;
				flex: 1 1 0%;
			}

			video {
				object-fit: contain;
			}
		</style>

		<title>Typing At Computers</title>
	</head>
	<body>
		<nav class="nav-bar">
			<a href="/">Home</a>

			{{range $i, $e := .routes}}
			{{if .Alias}}
			<a href="{{.Path}}">{{.Alias}}</a>
			{{end}}
			{{end}}
		</nav>
		<form class="search-area" action="/search" method="get">
			<input type="text" class="search-box" name="terms" id="search-terms" value="{{.terms}}" required>
			<input type="hidden" name="sortcriteria" id="sort-criteria" value="diskfilename">
			<input type="hidden" name="sortorder" id="sort-order" value="random">
			<input type="hidden" name="pagenumber" id="page-number" value="0">
			<input class="big-button" type="submit" value="Search">
		</form>

		{{template "body" .}}
	</body>
</html>
`,
	"template": `
	{{range $idx, $elem := .element}}
	<form class="mega-flexy-col" id="form-{{index $elem 0}}" action="add" method="post">
		<h2>Template Editor: {{index $elem 0}}</h2>
		<textarea class="big-text-box" name="raw">{{index $elem 1}}</textarea>
		<input type="hidden" name="name" value="{{index $elem 0}}">
		<input type="hidden" name="previous" value="{{index $elem 2}}">
		<input class="big-button" type="submit" value="Update">
	</form>
	{{end}}
`,
	"templates": `
{{range $idx, $elem := .elements}}
	<form class="flexy-col" id="form-{{index $elem 0}}" action="edit" method="get">
		<input type="hidden" name="name" value="{{index $elem 0}}">
		<input class="big-button" type="submit" value="{{index $elem 0}}">
	</form>
{{end}}
`,
	"video": `
	{{if eq .mediatype "video"}}
	<video controls>
		<source src="/file/{{.diskfilename | escapepath}}">
		<a href="/file/{{.diskfilename | escapepath}}">Download</a>
	</video>
	{{else if eq .mediatype "audio"}}
	<audio controls>
		<source src="/file/{{.filename | escapepath}}">
		<a href="/file/{{.filename | escapepath}}">Download</a>
	</audio>
	{{else if eq $.mediatype "image"}}
	<img src="/file/{{.filename | escapepath}}"/>
	{{else}}
	<h3>Unknown media type "{{.mediatype}}"</h3>
	{{end}}

	<div class="video-title">
		<h1>{{if .title}}{{.title}}{{else}}{{.filename}}{{end}}</h1>
	</div>

	{{if eq .favourite "true"}}
	<form id="remove-from-favourites" action="/favourites/remove" method="post">
		<input type="hidden" name="filename" value="{{.filename}}">
		<input class="big-button" type="submit" value="Remove from Favourites">
	</form>
	{{else}}
	<form id="add-to-favourites" action="/favourites/add" method="post">
		<input type="hidden" name="filename" value="{{.filename}}">
		<input class="big-button" type="submit" value="Add to Favourites">
	</form>
	{{end}}

	<div class="video-description">
		{{if .artist}}<h2>Created by <a href="/search?terms=artist:&quot;{{.artist}}&quot;">{{.artist}}</a></h2>{{end}}
		{{if .date}}
		<h2>Published on {{.date}}</h2>
		{{else}}
		<h2>File date is {{.diskfiletime}}</h2>
		{{end}}
		{{if .description}}<pre>{{.description}}</pre>{{end}}
	</div>

	<h1>Related Videos</h1>
	<ol>{{range $k, $vs := .related}}
		<li><a href="/watch?filename={{index $vs 0 | escapequery}}"><img src="/tmb/{{index $vs 1 | escapepath}}"/><h2>{{index $vs 0 | prettyprint}}</h2></a></li>
	{{end}}</ol>

	{{if 0}}
	<h1>Page Data</h1>
	<table>
		<tbody>
		{{range $k, $v := .}}
			<tr>
				<td>{{$k}}</td>
				<td>{{$v}}</td>
			</tr>
		{{end}}
		</tbody>
	</table> 
	{{end}}
`,
	"index": `
{{if .issearch}}
<div class="refinement-area">

<div class="search-refinements">
	<div>Sort By</div>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="diskfilename">
		<input type="hidden" name="sortorder" id="sort-order" value="desc">
		<input type="hidden" name="pagenumber" value="0">
		<input {{if eq $.sortcriteria "diskfilename"}}class="selected"{{end}} type="submit" value="Filename">
	</form>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="diskfiletime">
		<input type="hidden" name="sortorder" id="sort-order" value="desc">
		<input type="hidden" name="pagenumber" value="0">
		<input {{if eq $.sortcriteria "diskfiletime"}}class="selected"{{end}} type="submit" value="Date/Time">
	</form>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="duration">
		<input type="hidden" name="sortorder" id="sort-order" value="desc">
		<input type="hidden" name="pagenumber" value="0">
		<input {{if eq $.sortcriteria "duration"}}class="selected"{{end}} type="submit" value="Duration">
	</form>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="diskfilesize">
		<input type="hidden" name="sortorder" id="sort-order" value="desc">
		<input type="hidden" name="pagenumber" value="0">
		<input {{if eq $.sortcriteria "diskfilesize"}}class="selected"{{end}} type="submit" value="Filesize">
	</form>
</div>

<div class="search-refinements">
	<div>Sort Order</div>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="{{$.sortcriteria}}">
		<input type="hidden" name="sortorder" id="sort-order" value="asc">
		<input type="hidden" name="pagenumber" value="0">
		<input {{if eq $.sortorder "asc"}}class="selected"{{end}}type="submit" value="Ascending">
	</form>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="{{$.sortcriteria}}">
		<input type="hidden" name="sortorder" id="sort-order" value="desc">
		<input type="hidden" name="pagenumber" value="0">
		<input {{if eq $.sortorder "desc"}}class="selected"{{end}}type="submit" value="Descending">
	</form>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="{{$.sortcriteria}}">
		<input type="hidden" name="sortorder" id="sort-order" value="random">
		<input type="hidden" name="pagenumber" value="0">
		<input {{if eq $.sortorder "random"}}class="selected"{{end}}type="submit" value="Random">
	</form>
</div>

{{if .refinements}}
<div class="search-refinement-words">
	<div>Refinements</div>

	<div class="refine-words">
	{{range $i, $e := .refinements}}
	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}} {{$e}}" required>
		<input type="hidden" name="sortcriteria" value="{{$.sortcriteria}}">
		<input type="hidden" name="sortorder" id="sort-order" value="{{$.sortorder}}">
		<input type="hidden" name="pagenumber" value="{{$.pagenumber}}">
		<input type="submit" value="{{$e}}">
	</form>
	{{end}}
	</div>
</div>
{{end}}

{{if 0}}
<div class="search-refinements">
	<div>Page: {{$.pagenumber}}</div>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="{{$.sortcriteria}}">
		<input type="hidden" name="sortorder" id="sort-order" value="{{$.sortorder}}">
		<input type="hidden" name="pagenumber" value="{{$.nextpagenumber}}">
		<input type="submit" value="Next">
	</form>
</div>
{{end}}
</div>
{{end}}

<div class="thumbs">
{{range $idx, $elem := .videos}}
	<a class="media-item" href="/watch?filename={{index $elem 0 | escapequery}}{{if $.terms}}&terms={{$.terms}}{{end}}">
		<img class="thumb-img" src="/tmb/{{index $elem 1 | escapepath}}"/>
		<div class="media-title">{{index $elem 0 | prettyprint}}</div>
	</a>
{{end}}
</div>

{{if .issearch}}
<div class="bottom-zone">
	<div class="search-refinements">
		<div>Page</div>

		<form action="/search" method="get">
			<input type="hidden" name="terms" value="{{$.terms}}" required>
			<input type="hidden" name="sortcriteria" value="{{$.sortcriteria}}">
			<input type="hidden" name="sortorder" id="sort-order" value="{{$.sortorder}}">
			<input type="hidden" name="pagenumber" value="{{$.nextpagenumber}}">
			<input type="submit" value="Next">
		</form>
	</div>
</div>
{{end}}
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
}

