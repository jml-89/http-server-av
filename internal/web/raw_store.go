package web

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

	"/templates/edit": {
		"method":   "get",
		"template": "template",
	},

	"/templates/add": {
		"method":   "post",
		"redirect": "/templates/",
	},
	
	"/duplicates/": {
		"alias": "Duplicates",
		"method": "get",
		"template": "duplicates",
	},
}

var routeDefaultValues = map[string]map[string]string{
	"/search": {
		"pagenumber":   "0",
		"sortcriteria": "diskfiletime",
		"sortorder":    "desc",
		"issearch":     "1",
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

	"/templates/edit": {
		"element": `
			select name, raw, rowid 
			from templates 
			where name = :name
			and rowid not in (
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
				count(b.word) < sqrt(num.cnt)
			and 
				count(b.word) > (sqrt(num.cnt) / 2)
			order by
				random()
			limit 
				10;
		`,
	},

	"/favourites/add": {
		"query": `
			insert or replace into
			tags (filename, name, val)
			values (:filename, 'favourite', 'true');
		`,
	},

	"/favourites/remove": {
		"query": `
			insert or replace into
			tags (filename, name, val)
			values (:filename, 'favourite', 'false');
		`,
	},

	"/duplicates/": {
		"dupes": `
			select val, count(*)
			from tags
			where name = 'thumbname'
			group by val
			having count(*) > 1;
		`,
	},
}
