exports.up = async function (knex) {
    await knex.schema.alterTable('providers', table => {
        table.string('type').notNullable().alter();
        table.string('name').notNullable().alter();
        table.string('external_id');
    })
}

exports.down = async function (knex) {
    await knex.schema.alterTable('providers', table => {
        table.string('type').nullable().alter();
        table.string('name').nullable().alter();
        table.dropColumn('external_id');
    })
}
