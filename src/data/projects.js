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
		category: '',
		img:require('@/assets/images/number-game.png'),
	},

	{
		id:2,
		title:"Dice Rolling Discord Bot in Go",
		category:'',
		img: require('@/assets/images/dice-roller.png'),
	},

	{
		id:3,
		title:"PDF Text Extractor",
		category:'',
		img:require('@/assets/images/text-extraction.png'),
	},

	{
		id:4,
		title:"Urban Definitions",
		category:'',
		img: require('@/assets/images/urban-def.png'),
	}

];

export default projects;
