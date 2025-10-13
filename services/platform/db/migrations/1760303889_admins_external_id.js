exports.up = async function (knex) {
    await knex.schema.alterTable('admins', table => {
        table.string('external_id');
    })
}

exports.down = async function (knex) {
    await knex.schema.alterTable('admins', table => {
        table.dropColumn('external_id');
    })
}
