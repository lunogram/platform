exports.up = async function(knex) {
    await knex.schema.table('subscriptions', function(table) {
        table.tinyint('is_public').defaultTo(1)
    })
}

exports.down = async function(knex) {
    await knex.schema.table('subscriptions', function(table) {
        table.dropColumn('is_public')
    })
}
