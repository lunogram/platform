exports.up = async function (knex) {
    await knex.schema.alterTable('campaigns', table => {
        table.uuid('provider_id').nullable().alter();
        table.uuid('subscription_id').nullable().alter();
        table.dropColumn('name');
    })
}

exports.down = async function (knex) {
    await knex.schema.alterTable('campaigns', table => {
        table.uuid('provider_id').notNullable().alter();
        table.uuid('subscription_id').notNullable().alter();
        table.string('name').nullable();
    })
}
