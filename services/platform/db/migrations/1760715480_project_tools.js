exports.up = async function (knex) {
    await knex.schema.alterTable('projects', table => {
        table.specificType('tools', 'text[]').nullable();
    })
}

exports.down = async function (knex) {
    await knex.schema.alterTable('projects', table => {
        table.dropColumn('tools');
    })
}
