package main

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
			flex-direction: column;
			align-items: stretch;
		}

		#top-bar > * {
			margin: 0.2rem;
		}

		#search-terms {
			width: 35rem;
		}

		#search-refinements {
			display: flex;
			flex-direction: row;
			align-items: stretch;
		}

		#search-refinements > * {
			margin: 0.1rem;
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

	<head>
		<title>Typing At Computers</title>
	</head>
	<body>
		<div id="top-bar">
			<div id="quick-links">
				{{range $i, $e := .routes}}
				{{if .Alias}}
				<a href="{{.Path}}">{{.Alias}}</a>
				{{end}}
				{{end}}
			</div>

			<form id="search-area" action="/search" method="get">
				<input type="text" name="terms" id="search-terms" value="{{.terms}}" required>
				<input type="hidden" name="sortcriteria" id="sort-criteria" value="diskfilename">
				<input type="hidden" name="sortorder" id="sort-order" value="random">
				<input type="hidden" name="pagenumber" id="page-number" value="0">
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

	<div id="video-title">
		<h1>{{if .title}}{{.title}}{{else}}{{.filename}}{{end}}</h1>
	</div>

	{{if .favourite}}
	<form id="remove-from-favourites" action="/favourites/remove" method="post">
		<input type="hidden" name="filename" value="{{.filename}}">
		<input type="submit" value="Remove from Favourites">
	</form>
	{{else}}
	<form id="add-to-favourites" action="/favourites/add" method="post">
		<input type="hidden" name="filename" value="{{.filename}}">
		<input type="submit" value="Add to Favourites">
	</form>
	{{end}}

	<div id="video-description">
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

</div>
`,
	"index": `
{{if .issearch}}
{{if .refinements}}
<h2>Refinements</h2>
<div id="search-refinements">
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
{{end}}

<h2>Sort By: {{$.sortcriteria}}</h2>
<div id="search-refinements">
	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="diskfilename">
		<input type="hidden" name="sortorder" id="sort-order" value="desc">
		<input type="hidden" name="pagenumber" value="0">
		<input type="submit" value="Filename">
	</form>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="diskfiletime">
		<input type="hidden" name="sortorder" id="sort-order" value="desc">
		<input type="hidden" name="pagenumber" value="0">
		<input type="submit" value="Date/Time">
	</form>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="duration">
		<input type="hidden" name="sortorder" id="sort-order" value="desc">
		<input type="hidden" name="pagenumber" value="0">
		<input type="submit" value="Duration">
	</form>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="diskfilesize">
		<input type="hidden" name="sortorder" id="sort-order" value="desc">
		<input type="hidden" name="pagenumber" value="0">
		<input type="submit" value="Filesize">
	</form>
</div>

<h2>Sort Order: {{$.sortorder}}</h2>
<div id="search-refinements">
	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="{{$.sortcriteria}}">
		<input type="hidden" name="sortorder" id="sort-order" value="asc">
		<input type="hidden" name="pagenumber" value="0">
		<input type="submit" value="Ascending">
	</form>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="{{$.sortcriteria}}">
		<input type="hidden" name="sortorder" id="sort-order" value="desc">
		<input type="hidden" name="pagenumber" value="0">
		<input type="submit" value="Descending">
	</form>

	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="{{$.sortcriteria}}">
		<input type="hidden" name="sortorder" id="sort-order" value="random">
		<input type="hidden" name="pagenumber" value="0">
		<input type="submit" value="Random">
	</form>
</div>


<h2>Page: {{$.pagenumber}}</h2>
<div id="search-refinements">
	<form action="/search" method="get">
		<input type="hidden" name="terms" value="{{$.terms}}" required>
		<input type="hidden" name="sortcriteria" value="{{$.sortcriteria}}">
		<input type="hidden" name="sortorder" id="sort-order" value="{{$.sortorder}}">
		<input type="hidden" name="pagenumber" value="{{$.nextpagenumber}}">
		<input type="submit" value="Next">
	</form>
</div>
{{end}}

<div id="thumbs">
{{range $idx, $elem := .videos}}
	<div class="media-item">
		<a href="/watch?filename={{index $elem 0 | escapequery}}{{if $.terms}}&terms={{$.terms}}{{end}}"><img src="/tmb/{{index $elem 1 | escapepath}}"/>
		<h2>{{index $elem 0 | prettyprint}}</h2></a>
	</div>
{{end}}
</div>

{{if .issearch}}
<div id="bottom-zone">
	<h2>Page</h2>
	<div id="search-refinements">
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
]
`,
}

// Search bindings
// Searches require special handling in code to build a query
// These are stored in database table 'templatesearches'
// Below shows that search route needs a query built called searchresults
// args to search sourced from HTTP query parameter "terms"
var routeDefaultSearches = map[string]map[string]SearchBundle{
	"/search": {
		"searchresults": {
			Arg: "terms",
		},
	},
}

// routeDefaults are inserted into database in table 'routes'
// Each URL can only have one method associated with it
//
// route : { method, template, alias, redirect }
// alias - neater name, useful for links
// template - the template to use (see routeDefaultQueries for data handling)
// method - get or post
// redirect - go here instead
//
// method "post" implies a redirect and no template
// method "get" implies at least a template
var routeDefaults = map[string]map[string]string{
	"/": {
		"method":   "get",
		"template": "index",
		"alias":    "Home",
	},

	"/search": {
		"method":   "get",
		"template": "index",
	},

	"/favourites/": {
		"alias":    "Favourites",
		"method":   "get",
		"redirect": "/search?terms=favourite:true",
	},

	"/favourites/remove": {
		"method":   "post",
		"redirect": "/favourites/",
	},

	"/favourites/add": {
		"method":   "post",
		"redirect": "/favourites/",
	},

	"/watch": {
		"method":   "get",
		"template": "video",
	},

	"/templates/": {
		"alias":    "Templates",
		"method":   "get",
		"template": "templates",
	},

	"/templates/add": {
		"method":   "post",
		"redirect": "/templates/",
	},
}

var routeDefaultValues = map[string]map[string]string{
	"/search": {
		"pagenumber": "0",
		"sortcriteria": "diskfiletime",
		"sortorder": "desc",
		"issearch": "1",
	},
}

// routeDefaultQueries are inserted into database table 'templatequeries'
// This table stores the SQL statements used to fill in data for templates
// :variable are sourced from request parameters
var routeDefaultQueries = map[string]map[string]string{
	"/watch": {
		"related": `
			with wordcount(filename, num) as (
				select 
					filename, count(*)
				from 
					wordassocs
				group by 
					filename
			), wordlist(word) as (
				select 
					word
				from 
					wordassocs
				where 
					filename is :filename
			), related(filename, commoncount) as (
				select 
					a.filename, 
					count(*)
				from 
					wordassocs a
				where 
					a.filename is not :filename
					and a.word in wordlist
				group by 
					a.filename 
				having 
					count(*) > 0
			), scored(filename, leftcount, rightcount, commoncount) as (
				select 
					a.filename,
					(select num from wordcount where filename is :filename),
					(select num from wordcount where filename is a.filename),
					a.commoncount
				from 
					related a
			), stage2(filename, thumb, score) as (
				select 
					a.filename,
					(select val from tags where name is 'thumbname' and filename is a.filename),
					(a.commoncount * 200) / (a.leftcount + a.rightcount)
				from 
					scored a
			)
			select 
				filename, thumb, score
			from 
				stage2
			order by
				score desc 
			limit
				25
			;
		`,
	},

	"/templates/": {
		"elements": `
			select name, raw, rowid 
			from templates 
			where rowid not in (
				select previous from templates);
			`,
	},

	"/templates/add": {
		"query": `
			insert or ignore into 
			templates (previous, name, raw) 
			values (:previous, :name, :raw);
		`,
	},

	"/": {
		"videos": `
			select filename, val 
			from tags 
			where name is 'thumbname' 
			order by rowid desc 
			limit 50;
		`,
	},

	"/search": {
		"videos": `
			with unsorted(filename, criteria) as (
				select filename, val
				from ({{searchresults}})
				where name is :sortcriteria
			)
			select b.filename, b.val 
			from unsorted a
			join tags b
			on a.filename = b.filename
			and b.name is 'thumbname'
			order by 
				case when :sortorder is 'desc' then a.criteria end desc, 
				case when :sortorder is 'asc' then a.criteria end asc, 
				case when :sortorder is 'random' then random() end 
			limit 50
			offset :pagenumber * 50;
		`,

		"nextpagenumber": `
			select :pagenumber + 1;
		`,

		"refinements": `
			with search(filename) as (
				select distinct(filename) 
				from ({{searchresults}})
			)
			select 
				b.word
			from 
				(select count(*) as cnt from search) num
			left outer join
				(select filename from search) a
			join
				wordassocs b
			on 
				a.filename = b.filename
			group by 
				b.word
			having 
				count(b.word) < (num.cnt/2)
			order by
				count(b.word) desc
			limit 
				10;
		`,
	},

	"/favourites/add": {
		"query": `
			insert or ignore into 
			tags (filename, name, val)
			values (:filename, 'favourite', 'true');
		`,
	},

	"/favourites/remove": {
		"query": `
			delete from tags 
			where filename is :filename
			and name is 'favourite';
		`,
	},
}
