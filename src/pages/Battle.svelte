<script>
    import Sockette from 'sockette'
    import { onMount, onDestroy } from 'svelte'

    import PageLayout from '../components/PageLayout.svelte'
    import PointCard from '../components/PointCard.svelte'
    import WarriorCard from '../components/WarriorCard.svelte'
    import BattlePlans from '../components/BattlePlans.svelte'
    import VotingControls from '../components/VotingControls.svelte'
    import InviteWarrior from '../components/InviteWarrior.svelte'
    import VoteResults from '../components/VoteResults.svelte'

    import { warrior } from '../stores.js'

    export let battleId
    export let notifications
    export let router

    const hostname = window.location.origin
    const socketExtension = window.location.protocol === 'https:' ? 'wss' : 'ws'

    let socketError = false
    let socketReconnecting = false
    let points = ['1', '2', '3', '5', '8', '13']
    let vote = ''
    let voteStartTime = new Date()
    let battle = {}
    let currentPlanName = '[Voting not started]'

    let currentTime = new Date()

    $: countdown = battle.currentPlanId !== '' && battle.votingLocked === false ? timeUnitsBetween(voteStartTime, currentTime) : {}

    const onSocketMessage = function (evt) {
        const parsedEvent = JSON.parse(evt.data)

        switch(parsedEvent.type) {
            case "init":
                battle = JSON.parse(parsedEvent.value)
                if (battle.activePlanId !== '') {
                    const activePlan = battle.plans.find(p => p.id === battle.activePlanId)
                    currentPlanName = activePlan.name
                    voteStartTime = new Date(activePlan.voteStartTime)
                }
                break;
            case "warrior_joined":
                battle.warriors = JSON.parse(parsedEvent.value)
                const joinedWarrior = battle.warriors.find(w => w.id === parsedEvent.warriorId)
                notifications.success(`${joinedWarrior.name} joined.`)
                break;
            case "warrior_retreated":
                const leftWarrior = battle.warriors.find(w => w.id === parsedEvent.warriorId)
                battle.warriors = JSON.parse(parsedEvent.value)
                notifications.danger(`${leftWarrior.name} retreated.`)
                break;
            case "plan_added":
                battle.plans = JSON.parse(parsedEvent.value)
                break;
            case "plan_activated":
                const updatedPlans = JSON.parse(parsedEvent.value)
                const activePlan = updatedPlans.find(p => p.active)
                currentPlanName = activePlan.name
                voteStartTime = new Date(activePlan.voteStartTime)

                battle.plans = updatedPlans
                battle.activePlanId = activePlan.id
                battle.votingLocked = false
                vote = ''
                break;
            case "plan_skipped":
                const updatedPlans2 = JSON.parse(parsedEvent.value)
                currentPlanName = '[Voting not started]'
                battle.plans = updatedPlans2
                battle.activePlanId = ''
                battle.votingLocked = true
                vote = ''
                notifications.warning(`Plan skipped.`)
                break;
            case "vote_activity":
                const votedWarrior = battle.warriors.find(w => w.id === parsedEvent.warriorId)
                notifications.success(`${votedWarrior.name} voted.`)

                battle.plans = JSON.parse(parsedEvent.value)
                break;
            case "vote_retracted":
                const devotedWarrior = battle.warriors.find(w => w.id === parsedEvent.warriorId)
                notifications.warning(`${devotedWarrior.name} retracted vote.`)

                battle.plans = JSON.parse(parsedEvent.value)
                break;
            case "voting_ended":
                battle.plans = JSON.parse(parsedEvent.value)
                battle.votingLocked = true
                break;
            case "plan_finalized":
                battle.plans = JSON.parse(parsedEvent.value)
                battle.activePlanId = ''
                currentPlanName = '[Voting not started]'
                vote = ''
                break;
            case "plan_revised":
                battle.plans = JSON.parse(parsedEvent.value)
                break;
            case "plan_burned":
                const postBurnPlans = JSON.parse(parsedEvent.value)

                if (battle.activePlanId !== '' && postBurnPlans.filter(p => p.id === battle.activePlanId).length === 0) {
                    battle.activePlanId = ''
                    currentPlanName = '[Voting not started]'
                }

                battle.plans = postBurnPlans

                break;
            case "battle_updated":
                battle = JSON.parse(parsedEvent.value)
                break;
            case "battle_conceded":
                // battle over, goodbye.
                router.route('/')
                break;
            case "jab_warrior":
                const warriorToJab = battle.warriors.find(w => w.id === parsedEvent.value)
                notifications.info(`pst... ${warriorToJab.name}, waiting on you to vote.`)
                break;
            default:
                break;
        }
    }

    const ws = new Sockette(`${socketExtension}://${window.location.host}/api/arena/${battleId}`, {
        timeout: 2e3,
        maxAttempts: 15,
        onmessage: onSocketMessage,
        onerror:  () => {
            socketError = true
        },
        onclose: () => {
            socketReconnecting = true
        },
        onopen: () => {
            socketError = false
            socketReconnecting = false
        },
        onmaximum: () => {
            socketReconnecting = false
        }
    })

    onDestroy(() => {
        ws.close();
    })

    const sendSocketEvent = (type, value) => {
        ws.send(JSON.stringify({
            type,
            value
        }))
    }

    const handleVote = (event) => {
        vote = event.detail.point
        const voteValue = {
            planId: battle.activePlanId,
            voteValue: vote
        }

        sendSocketEvent('vote', JSON.stringify(voteValue))
    }

    const handleUnvote = () => {
        vote = ''

        sendSocketEvent('retract_vote', battle.activePlanId)
    }

    // Determine if the warrior has voted on active Plan yet
    function didVote(warriorId) {
        if (battle.activePlanId === "") {
            return false
        }
        const currentPlan = battle.plans.find(p => p.id === battle.activePlanId)
        const voted = currentPlan.votes.find(w => w.warriorId === warriorId)

        return voted !== undefined
    }

    // Determine if we are showing warriors vote
    function showVote(warriorId) {
        if (battle.activePlanId === "" || battle.votingLocked === false) {
            return ''
        }
        const currentPlan = battle.plans.find(p => p.id === battle.activePlanId)
        const voted = currentPlan.votes.find(w => w.warriorId === warriorId)

        return voted !== undefined ? voted.vote : ''
    }

    // get hightest vote from active plan
    function getHighestVote() {
        const voteCounts = {}
        points.forEach(p => {
            voteCounts[p] = 0
        })
        const highestVote = {
            vote: '',
            count: 0
        }
        const activePlan = battle.plans.find(p => p.id === battle.activePlanId)

        if (activePlan.votes.length > 0) {
            const reversedPoints = [...points].filter(v => v !== '?').reverse()
            reversedPoints.push('?')

            // build a count of each vote
            activePlan.votes.forEach(v => {
                ++voteCounts[v.vote]
            })

            // find the highest vote giving priority to higher numbers
            reversedPoints.forEach(p => {
                if (voteCounts[p] > highestVote.count) {
                    highestVote.vote = p
                    highestVote.count = voteCounts[p]
                }
            })
        }

        return highestVote.vote
    }
    $: highestVoteCount = battle.activePlanId !== '' && battle.votingLocked === true ? getHighestVote() : ''

    function concedeBattle() {
        sendSocketEvent("concede_battle", "")
    }

    function timeUnitsBetween(startDate, endDate) {
        let delta = Math.abs(endDate - startDate) / 1000
        return [
            ['days', 24 * 60 * 60],
            ['hours', 60 * 60],
            ['minutes', 60],
            ['seconds', 1]
        ].reduce((acc, [key, value]) => (acc[key] = Math.floor(delta / value), delta -= acc[key] * value, acc), {})
    }

    function addTimeLeadZero(time) {
        return ('0' + time).slice(-2)
    }

    onMount(() => {
        if (!$warrior.id) {
            router.route(`/enlist/${battleId}`)
        }
		const voteCounter = setInterval(() => {
			currentTime = new Date()
		}, 1000)

		return () => {
			clearInterval(voteCounter)
		};
	});
</script>

<svelte:head>
    <title>Battle {battle.name} | Thunderdome</title>
</svelte:head>

<PageLayout>
    {#if battle.name && !socketReconnecting && !socketError}
        <div class="mb-6 flex flex-wrap">
            <div class="w-full text-center md:w-2/3 md:text-left">
                <h1>
                    <a href="{currentPlanName}" target="_blank">{currentPlanName}</a>
                </h1>
                <h2 class="text-grey-darker">{battle.name}</h2>
            </div>
            <div class="w-full md:w-1/3 text-center md:text-right font-semibold text-3xl md:text-4xl text-grey-darker" data-testId="votingTimer">
                {#if countdown.seconds !== undefined}
                    {#if countdown.hours !== 0}{addTimeLeadZero(countdown.hours)}:{/if}{addTimeLeadZero(countdown.minutes)}:{addTimeLeadZero(countdown.seconds)}
                {/if}
            </div>
        </div>

        <div class="flex flex-wrap mb-4 -mx-4">
            <div class="w-full lg:w-3/4 px-4">
                {#if battle.activePlanId !== '' && battle.votingLocked === true}
                    <VoteResults plans={battle.plans} activePlanId={battle.activePlanId} points={points} />
                {:else}
                    <div class="flex flex-wrap mb-4 -mx-2 mb-4 lg:mb-6">
                        {#each points as point}
                            <div class="w-1/4 md:w-1/6 px-2 mb-4">
                                <PointCard
                                    point={point}
                                    active={vote === point}
                                    on:voted={handleVote}
                                    on:voteRetraction={handleUnvote}
                                    isLocked={battle.votingLocked}
                                />
                            </div>
                        {/each}
                    </div>
                {/if}

                <BattlePlans
                    plans={battle.plans}
                    isLeader={battle.leaderId === $warrior.id}
                    sendSocketEvent={sendSocketEvent}
                />
            </div>

            <div class="w-full lg:w-1/4 px-4">
                <div class="bg-white shadow-md mb-4 rounded">
                    <div class="bg-blue p-4 rounded-t">
                        <h3 class="text-2xl text-white">Warriors</h3>
                    </div>

                    {#each battle.warriors as war (war.id)}
                        <WarriorCard
                            warrior={war}
                            leaderId={battle.leaderId}
                            isLeader={battle.leaderId === $warrior.id}
                            voted={didVote(war.id)}
                            points={showVote(war.id)}
                            sendSocketEvent={sendSocketEvent}
                        />
                    {/each}

                    {#if battle.leaderId === $warrior.id}
                        <VotingControls
                            points={points}
                            planId={battle.activePlanId}
                            sendSocketEvent={sendSocketEvent}
                            votingLocked={battle.votingLocked}
                            highestVote={highestVoteCount}
                        />
                    {/if}
                </div>

                <div class="bg-white shadow-md p-5 mb-4 rounded">
                    <InviteWarrior hostname={hostname} battleId={battle.id} />
                    {#if battle.leaderId === $warrior.id}
                        <div class="mt-4 text-right">
                            <button
                                class="bg-transparent hover:bg-red text-red-dark font-semibold hover:text-white py-2 px-2 border border-red hover:border-transparent rounded"
                                on:click={concedeBattle}
                            >
                                Delete Battle
                            </button>
                        </div>
                    {/if}
                </div>
            </div>
        </div>
    {:else if socketReconnecting}
        <div class="flex items-center">
            <div class="flex-1 text-center">
                <h1 class="text-5xl text-teal">Ooops, reloading Battle Plans...</h1>
            </div>
        </div>
    {:else if socketError}
        <div class="flex items-center">
            <div class="flex-1 text-center">
                <h1 class="text-5xl text-red">Error joining battle, refresh and try again.</h1>
            </div>
        </div>
    {:else}
        <div class="flex items-center">
            <div class="flex-1 text-center">
                <h1 class="text-5xl text-teal">Loading Battle Plans...</h1>
            </div>
        </div>
    {/if}
</PageLayout>
