<script>
export default {
	setup() {
		return {
			bios: [
				{
					id: 1,
					bio:
					"I'm Christian, a software engineer currently living in the Atlanta area. I've been studying programming for 6 years now, and I'm looking for an opportunity to contribute to your team."
				},
				{
					id: 2,
					bio:
					"While this site is written in Vue and Tailwind, my real strengths lie in Python and Go. I have an interest in cloud and microservices architecture, and a passion for writing idiomatic, efficient and beautiful code."
				},
				{
					id: 3,
					bio:
					"In my spare time, I really enjoy cooking and video games. "
				},
				{
					id: 4,
					bio:
					"Thank you for checking out my profile - here's a download for my resume below. I hope you consider making me a part of your team!"
				},
			],
		};
	},
};
</script>

<template>
	<div class="block sm:flex sm:gap-10 mt-10 sm:mt-20">
		<!-- About profile image -->
		<div class="w-full sm:w-1/4 mb-7 sm:mb-0">
			<img
				src="@/assets/images/profile.jpeg"
				class="rounded-xl w-96"
				alt=""
			/>
		</div>

		<!-- About details -->
		<div class="w-full sm:w-3/4 text-left">
			<p
				v-for="bio in bios"
				:key="bio.id"
				class="font-Inter mb-4 text-ternary-dark dark:text-ternary-light text-xl align-middle"
			>
				{{ bio.bio }}
			</p>
				<a
					download="Christian-Kulat-Resume.pdf"
					href="/files/Christian-Kulat-Resume-Feb2023.pdf"
					class="flex justify-center items-center w-36 sm:w-48 mt-12 mb-6 sm:mb-0 text-lg border border-indigo-200 dark:border-ternary-dark py-2.5 sm:py-3 shadow-lg rounded-lg bg-indigo-50 focus:ring-1 focus:ring-sky hover:bg-blue-400 text-gray-500 hover:text-white duration-500"
					aria-label="Download Resume"
				>
					<i
						data-feather="arrow-down-circle"
						class="ml-0 sm:ml-1 mr-2 sm:mr-3 w-5 sm:w-6 duration-100"
					></i>
					<span
						class="text-xs sm:text-xs font-general-medium duration-100"
						>Here's my resume.</span
					></a
				>
			</div>
		</div>
</template>
