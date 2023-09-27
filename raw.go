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
	<body>
		<div id="top-bar">
			<div id="quick-links">
				{{range $i, $e := .routes}}
				<a href="{{.Path}}">{{.Alias}}</a>
				{{end}}
			</div>

			<form id="search-area" action="/search" method="get">
				<input type="text" name="terms" id="search-terms" value="{{.terms}}" required>
				<input type="hidden" name="sortcriteria" id="sort-criteria" value="diskfilename">
				<input type="hidden" name="sortorder" id="sort-order" value="desc">
				<input type="hidden" name="pagenumber" id="page-number" value="0">
				<input type="submit" value="Search">
			</form>

			{{if .issearch}}
			{{if .refinements}}
			<h2>Refinements</h2>
			<div id="search-refinements">
				{{range $i, $e := .refinements}}
				<form action="/search" method="get">
					<input type="hidden" name="terms" value="{{$.terms}} {{$e}}" required>
					<input type="hidden" name="sortcriteria" value="{{$.sortcriteria}}">
					<input type="hidden" name="sortorder" id="sort-order" value="desc">
					<input type="hidden" name="pagenumber" value="{{$.pagenumber}}">
					<input type="submit" value="{{$e}}">
				</form>
				{{end}}
			</div>
			{{end}}

			<h2>Sort By</h2>
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

			<h2>Sort Order</h2>
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


			<h2>Page</h2>
			<div id="search-refinements">
				<form action="/search" method="get">
					<input type="hidden" name="terms" value="{{$.terms}}" required>
					<input type="hidden" name="sortcriteria" value="{{$.sortcriteria}}">
					<input type="hidden" name="sortorder" id="sort-order" value="desc">
					<input type="hidden" name="pagenumber" value="{{index $.nextpagenumber 0}}">
					<input type="submit" value="Next">
				</form>
			</div>
			{{end}}
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
	{{if eq (index .mediatype 0) "video"}}
	<video controls>
		<source src="/file/{{index .filename 0 | escapepath}}">
		<a href="/file/{{index .filename 0 | escapepath}}">Download</a>
	</video>
	{{else if eq (index .mediatype 0) "audio"}}
	<audio controls>
		<source src="/file/{{index .filename 0 | escapepath}}">
		<a href="/file/{{index .filename 0 | escapepath}}">Download</a>
	</audio>
	{{else if eq (index .mediatype 0) "image"}}
	<img src="/file/{{index .filename 0 | escapepath}}"/>
	{{else}}
	<h3>Unknown media type "{{index .mediatype 0}}"</h3>
	{{end}}

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
		<li><a href="/watch?filename={{index $vs 0 | escapequery}}"><img src="/tmb/{{index $vs 1 | escapepath}}"/><h2>{{index $vs 0 | prettyprint}}</h2></a><h3>{{index $vs 2}}</h3></li>
	{{end}}</ol>
</div>
`,
	"index": `
<div id="thumbs">
{{range $idx, $elem := .videos}}
	<div class="media-item">
		<a href="/watch?filename={{index $elem 0 | escapequery}}{{if $.terms}}&terms={{$.terms}}{{end}}"><img src="/tmb/{{index $elem 1 | escapepath}}"/>
		<h2>{{index $elem 0 | prettyprint}}</h2></a>
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
		"alias": "Home",
	},

	"/search": {
		"method":   "get",
		"template": "index",
	},

	"/favourites/": {
		"alias":    "Favourites",
		"method":   "get",
		"template": "index",
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

// routeDefaultQueries are inserted into database table 'templatequeries'
// This table stores the SQL statements used to fill in data for templates
// :variable are sourced from request parameters
var routeDefaultQueries = map[string]map[string]string{
	"/watch": {
		"filename": `
			select distinct(filename) 
			from tags 
			where filename is :filename;
		`,

		"mediatype": `
			select val 
			from tags 
			where filename is :filename 
			and name is 'mediatype';
		`,

		"title": `
			select val 
			from tags 
			where filename is :filename 
			and name is 'title';
		`,

		"duration": `
			select val 
			from tags 
			where filename is :filename 
			and name is 'duration';
		`,

		"artist": `
			select val 
			from tags 
			where filename is :filename 
			and name is 'artist';
		`,

		"tags": `
			select name, val 
			from tags 
			where filename is :filename;
		`,

		"favourite": `
			select filename 
			from favourites 
			where filename is :filename;
		`,

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

		"issearch": `
			select 1;
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

	"/favourites/": {
		"videos": `
			select a.filename, a.val 
			from favourites b join tags a 
			on a.filename = b.filename 
			where a.name is 'thumbname'
			order by b.rowid desc;
		`,
	},

	"/favourites/add": {
		"query": `
			insert or ignore into 
				favourites (filename) 
				values (:filename);
		`,
	},

	"/favourites/remove": {
		"query": `
			delete from favourites 
			where filename is :filename;
		`,
	},
}

