package main

const tpl = `
<html>
<head></head>
<body>
{{.}}
	<header>hey, here are your stats</header>
	<container>
	<content>
	{{range .stats}}
		<div class="row"><div class="col-sm-4"> {{.Took}} </div>
	<div class="col-sm-8"> [ {{.Request.RemoteAddr}} ] </div>
	</div>
		
	{{end}}
	</content>
	<aside>side</aside>
</container>
done
</body>
</html>
`
