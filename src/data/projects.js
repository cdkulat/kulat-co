// Projects data.
// Note: Here we are getting data from a js file, but in a different project I'll be fetching these projects from some srt of APIs.

const projects = [
	//Formatting guide:

	/*{
		id: 1,
		title: 'Google Health Platform',
		category: 'Web Application',
		img: require('@/assets/images/web-project-2.jpg'),
	}, */

	{
		id:1,
		title:'Number Guessing Game',
		category: 'Command Line Application',
		img:require('@/assets/images/number-game.png'),
		technologies: "Python",
		objective: '',
		description: '',
	},

	{
		id:2,
		title:"Dice Rolling Discord Bot in Go",
		category:'Bots',
		img: require('@/assets/images/dice-roller.png'),
		technologies: "Go, Discord",
		objective: '',
		description: '',
	},

	{
		id:3,
		title:"PDF Text Extractor",
		category:'Command Line Application',
		img:require('@/assets/images/text-extraction.png'),
		technologies: 'Python',
		objective: '',
		description: '',
	},

	{
		id:4,
		title:"Urban Definitions",
		category:'Bots',
		img: require('@/assets/images/urban-def.png'),
		technologies: 'Go, Discord',
		objective: '',
		description: '',
	}

];

export default projects;
