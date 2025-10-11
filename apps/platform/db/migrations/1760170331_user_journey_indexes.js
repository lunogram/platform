exports.up = async function (knex) {
    await knex.schema.alterTable('journey_user_step', table => {
        table.index(['entrance_id'], 'user_journey_step_entrance_id_index')
        table.index(['journey_id'], 'user_journey_step_journey_id_index')
    })
}

exports.down = async function (knex) {
    await knex.schema.alterTable('journey_user_step', table => {
        table.dropIndex(['entrance_id'], 'user_journey_step_entrance_id_index')
        table.dropIndex(['journey_id'], 'user_journey_step_journey_id_index')
    })
}
