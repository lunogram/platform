exports.up = async function (knex) {
    await knex.schema.alterTable('organizations', table => {
        table.dropColumn('domain');
        table.dropColumn('username');
        table.string('name').notNullable().defaultTo('');
    })
}

exports.down = async function (knex) {
    await knex.schema.alterTable('organizations', table => {
        table.dropColumn('name');
        table.string('domain').notNullable().defaultTo('');
        table.string('username').notNullable().defaultTo('');
    })
}
